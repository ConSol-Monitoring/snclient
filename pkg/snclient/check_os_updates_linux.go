package snclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	reAPTSecurity = regexp.MustCompile(`(Debian-Security:|Ubuntu:[^/]*/[^-]*-security)`)
	reAPTEntry    = regexp.MustCompile(`^Inst\s+(\S+)\s+\[([^\[]+)\]\s+\((\S+)\s+(.*)\s+\[(\S+)\]\)`)
	reYUMEntry    = regexp.MustCompile(`^(\S+)\.(\S+)\s+(\S+)\s+(\S+)`)
)

func (l *CheckOSUpdates) addOSBackends(ctx context.Context, check *CheckData) (int, error) {
	addedCount := 0
	var err error
	err = nil

	aptAdded, aptErr := l.addAPT(ctx, check)
	if aptAdded {
		addedCount++
	}
	if aptErr != nil {
		err = fmt.Errorf("error when adding apt: %w", aptErr)
	}

	yumAdded, yumErr := l.addYUM(ctx, check)
	if yumAdded {
		addedCount++
	}
	if yumErr != nil {
		if err == nil {
			err = fmt.Errorf("error when adding yum: %w", yumErr)
		} else {
			err = fmt.Errorf("%w | error when adding yum: %w", err, yumErr)
		}
	}

	return addedCount, err
}

// get packages from apt
func (l *CheckOSUpdates) addAPT(ctx context.Context, check *CheckData) (bool, error) {
	switch l.system {
	case "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/apt-get")
		if os.IsNotExist(err) {
			return false, nil
		}
	case "apt":
	default:
		return false, nil
	}

	aptOpts := " -o 'Debug::NoLocking=true'"
	if l.update {
		cacheDir, err := l.pkgListsDir("apt", []string{"partial"})
		if err != nil {
			return true, err
		}

		if cacheDir != "" {
			aptOpts += fmt.Sprintf(" -o Dir::State::Lists=%q", cacheDir)
		}

		updateOpts := aptOpts + " -o 'APT::Update::Error-Mode=any'"
		output, stderr, rc, err := l.snc.execCommand(ctx, "apt-get update"+updateOpts, l.snc.getBuiltinCmdTimeout())
		if err != nil {
			return true, fmt.Errorf("apt-get update failed: %s\n%s", err.Error(), stderr)
		}
		if rc != 0 {
			return true, fmt.Errorf("apt-get update failed: %s\n%s", output, stderr)
		}
	}

	output, stderr, rc, err := l.snc.execCommand(ctx, "apt-get upgrade"+aptOpts+" -s -qq", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return true, fmt.Errorf("apt-get upgrade failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return true, fmt.Errorf("apt-get upgrade failed: %s\n%s", output, stderr)
	}

	l.parseAPT(output, check)

	return true, nil
}

// pkgListsDir returns a path to a private cache folder for package manager
// metadata. It creates the folder if it does not exist and ensures that it has
// proper permissions, user and is not a symlink.
// It returns an empty string if no cache folder is required.
func (l *CheckOSUpdates) pkgListsDir(subRoot string, subFolder []string) (string, error) {
	// root does not require cache folder
	if os.Geteuid() == 0 {
		return "", nil
	}
	cacheDir := l.snc.getCacheFolder()
	if err := os.MkdirAll(cacheDir, DefaultCacheFolderPermissions); err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}
	dir := cacheDir

	components := append([]string{subRoot}, subFolder...)
	for _, component := range components {
		dir = filepath.Join(dir, component)
		if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("failed to create pkg cache directory %s: %w", dir, err)
		}

		info, err := os.Lstat(dir)
		if err != nil {
			return "", fmt.Errorf("failed to inspect pkg cache directory %s: %w", dir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("pkg cache path %s is not a directory", dir)
		}
		if err := l.snc.checkFileOwner(dir); err != nil {
			return "", fmt.Errorf("invalid pkg cache directory: %w", err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", fmt.Errorf("failed to secure pkg cache directory %s: %w", dir, err)
		}
	}

	return filepath.Join(cacheDir, subRoot), nil
}

func (l *CheckOSUpdates) parseAPT(output string, check *CheckData) {
	for line := range strings.SplitSeq(output, "\n") {
		matches := reAPTEntry.FindStringSubmatch(line)
		security := "0"
		if reAPTSecurity.MatchString(line) {
			security = "1"
		}
		if len(matches) < 5 {
			continue
		}
		check.listData = append(check.listData, map[string]string{
			"security":    security,
			"package":     matches[1],
			"version":     matches[3],
			"old_version": matches[2],
			"repository":  matches[4],
			"arch":        matches[5],
		})
	}
}

// get packages from yum
func (l *CheckOSUpdates) addYUM(ctx context.Context, check *CheckData) (bool, error) {
	switch l.system {
	case "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/yum")
		if os.IsNotExist(err) {
			return false, nil
		}
	case "yum":
	default:
		return false, nil
	}

	// normally answer from cache only
	yumOpts := " --cacheonly"

	if l.update {
		cacheDir, err := l.pkgListsDir("dnf", nil)
		if err != nil {
			return true, err
		}

		if cacheDir != "" {
			yumOpts = fmt.Sprintf(" --setopt=cachedir=%q", cacheDir)
		}

		// Expiring the private cache before the query forces a metadata refresh
		// and works with both legacy Yum 3 and DNF.
		output, stderr, exitCode, cacheErr := l.snc.execCommand(ctx, "yum"+yumOpts+" clean expire-cache -q", l.snc.getBuiltinCmdTimeout())
		if cacheErr != nil {
			return true, fmt.Errorf("yum cache expiration failed: %s\n%s", cacheErr.Error(), stderr)
		}
		if exitCode != 0 {
			return true, fmt.Errorf("yum cache expiration failed: %s\n%s", output, stderr)
		}
	}
	yumOpts += " --setopt='*.skip_if_unavailable=False'"

	output, stderr, exitCode, err := l.snc.execCommand(ctx, "yum"+yumOpts+" check-update --security -q", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 && exitCode != 100 {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", output, stderr)
	}
	packageLookup := l.parseYUM(output, "1", check, nil)

	output, stderr, exitCode, err = l.snc.execCommand(ctx, "yum"+yumOpts+" check-update -q", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 && exitCode != 100 {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", output, stderr)
	}
	l.parseYUM(output, "0", check, packageLookup)

	return true, nil
}

func (l *CheckOSUpdates) parseYUM(output, security string, check *CheckData, skipPackages map[string]bool) map[string]bool {
	packages := map[string]bool{}
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "Obsoleting Packages") {
			break
		}
		matches := reYUMEntry.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		if skipPackages[matches[1]] {
			continue
		}
		packages[matches[1]] = true
		check.listData = append(check.listData, map[string]string{
			"security":    security,
			"package":     matches[1],
			"version":     matches[2],
			"old_version": "",
			"repository":  matches[3],
			"arch":        matches[2],
		})
	}

	return packages
}
