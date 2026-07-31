package snclient

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_mailq"] = CheckEntry{"check_mailq", NewCheckMailq}
}

var postQueueFolders = []string{"active", "deferred"}

type CheckMailq struct {
	snc *Agent
	mta string
}

func NewCheckMailq() CheckHandler {
	return &CheckMailq{
		mta: "auto",
	}
}

func (l *CheckMailq) Build() *CheckData {
	return &CheckData{
		name:         "check_mailq",
		description:  "Checks the mailq.",
		implemented:  Linux | FreeBSD | Darwin,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"mta": {value: &l.mta, isFilter: true, description: "Set source mta for checking mailq instead of auto detect. Can be postfix or auto"},
		},
		defaultFilter:   "none",
		defaultWarning:  "active > 5 || active_size > 10MB || deferred > 0 || deferred_size > 10MB",
		defaultCritical: "active > 10 || active_size > 20MB || deferred > 10 || deferred_size > 20MB",
		detailSyntax:    "${mta}: active ${active} / deferred ${deferred}",
		topSyntax:       "%(status) - ${list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - could not get any mailq data",
		attributes: []CheckAttribute{
			{name: "mta", description: "name of the mta"},
			{name: "folder", description: "checked spool folder"},
			{name: "active", description: "number of active mails"},
			{name: "active_size", description: "size of active mails in bytes", unit: UByte},
			{name: "deferred", description: "number of deferred mails"},
			{name: "deferred_size", description: "size of deferred mails in bytes", unit: UByte},
		},
		exampleDefault: `
    check_mailq
    OK - postfix: active 0 / deferred 0 |...
	`,
		exampleArgs: `warn='active > 5 || deferred > 0' crit='active > 10 || deferred > 10'`,
	}
}

func (l *CheckMailq) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	err := l.addQueues(ctx, check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}

func (l *CheckMailq) addQueues(ctx context.Context, check *CheckData) (err error) {
	if l.mta == "auto" || l.mta == "postfix" {
		err = l.postfixAdd(ctx, check)
		if err != nil {
			log.Debugf("failed: postfix: %s", err.Error())
			if l.mta != "auto" {
				return err
			}
		} else {
			return nil
		}
	}

	return err
}

func (l *CheckMailq) defaultEntry(source string) map[string]string {
	return map[string]string{
		"mta":           source,
		"active":        "",
		"active_size":   "",
		"deferred":      "",
		"deferred_size": "",
	}
}

// get queue stats from postfix
func (l *CheckMailq) postfixAdd(ctx context.Context, check *CheckData) error {
	queueFolder, stderr, rc, err := l.snc.execCommand(ctx, "postconf -h queue_directory", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("postfix: postconf failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("postconf failed: %s\n%s", queueFolder, stderr)
	}

	var entry map[string]string
	switch {
	case os.Geteuid() == 0:
		entry = l.postfixWalkQueueFolder(queueFolder)
	case HasCapabilities():
		entry, err = l.postfixRunFindQueueFolder(ctx, queueFolder)
		if err != nil {
			return err
		}
	}

	if entry != nil {
		check.listData = append(check.listData, entry)
		l.addMetrics(check, entry)
	}

	return nil
}

func (l *CheckMailq) addMetrics(check *CheckData, entry map[string]string) {
	if entry["active"] != "" {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:     "active",
				Unit:     "",
				Value:    convert.Int64(entry["active"]),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}

	if entry["active_size"] != "" {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:     "active_size",
				Unit:     "B",
				Value:    convert.Int64(entry["active_size"]),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}

	if entry["deferred"] != "" {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:     "deferred",
				Unit:     "",
				Value:    convert.Int64(entry["deferred"]),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}

	if entry["deferred_size"] != "" {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:     "deferred_size",
				Unit:     "B",
				Value:    convert.Int64(entry["deferred_size"]),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}
}

func (l *CheckMailq) postfixWalkQueueFolder(queueFolder string) (entry map[string]string) {
	entry = l.defaultEntry("postfix")
	for _, queue := range postQueueFolders {
		entry[queue] = "0"
		entry[queue+"_size"] = "0"

		srcPath := filepath.Join(queueFolder, queue)
		_, err := os.Stat(srcPath)
		if os.IsNotExist(err) {
			log.Debugf("checking folder %s failed: %s", queue, err.Error())

			continue
		}

		count, size, err := l.postfixFolderStats(srcPath)
		if err != nil {
			log.Debugf("checking folder %s failed: %s", queue, err.Error())

			entry["_error"] = err.Error()
		}
		entry[queue] = fmt.Sprintf("%d", count)
		entry[queue+"_size"] = fmt.Sprintf("%d", size)
	}

	return entry
}

func (l *CheckMailq) postfixFolderStats(folder string) (count, size int64, err error) {
	err = filepath.WalkDir(folder, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.Debugf("reading folder failed %s: %s", path, err.Error())

			return err
		}

		if dir.IsDir() {
			return nil
		}

		fileInfo, err := dir.Info()
		if err != nil {
			log.Debugf("reading file failed %s: %s", filepath.Join(path, dir.Name()), err.Error())

			return nil
		}

		count++
		size += fileInfo.Size()

		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("error walking directory %s: %w", folder, err)
	}

	return count, size, nil
}

func (l *CheckMailq) postfixRunFindQueueFolder(ctx context.Context, queueFolder string) (entry map[string]string, err error) {
	entry = l.defaultEntry("postfix")

	for _, queue := range postQueueFolders {
		entry[queue] = "0"
		entry[queue+"_size"] = "0"

		output, stderr, rc, err := l.snc.execCommandAsRoot(ctx, fmt.Sprintf("/usr/bin/find %q -type f -ls", filepath.Join(queueFolder, queue)), l.snc.getBuiltinCmdTimeout())
		if err != nil {
			return nil, fmt.Errorf("find failed: %s\n%s", err.Error(), stderr)
		}
		if rc != 0 {
			return nil, fmt.Errorf("find failed: %s\n%s", output, stderr)
		}

		totalSize := int64(0)
		count := 0
		for line := range strings.SplitSeq(output, "\n") {
			columns := strings.Fields(line)
			if len(columns) < 7 {
				continue
			}
			totalSize += convert.Int64(columns[6])
			count++
		}

		entry[queue] = fmt.Sprintf("%d", count)
		entry[queue+"_size"] = fmt.Sprintf("%d", totalSize)
	}

	return entry, nil
}
