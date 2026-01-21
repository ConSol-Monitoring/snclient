//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/disk"
)

func (l *CheckDrivesize) getDefaultFilter() string {
	return "fstype not in (" + utils.List2String(defaultExcludedFsTypes()) + ")"
}

func (l *CheckDrivesize) getExample() string {
	return `
    check_drivesize drive=/ show-all
    OK - / 280.155 GiB/455.948 GiB (64.7%) |...

Check drive including inodes:

    check_drivesize drive=/ warn="used > 90%" "crit=used > 95%" "warn=inodes > 90%" "crit=inodes > 95%"
    OK - All 1 drive(s) are ok |'/ used'=307515822080B;440613398938;465091921101;0;489570443264 '/ used %'=62.8%;90;95;0;100 '/ inodes'=12.1%;90;95;0;100

Check folder, no matter if its a mountpoint itself or not:

    check_drivesize folder=/tmp show-all
    OK - /tmp 280.155 GiB/455.948 GiB (64.7%) |...
	`
}

// if parentFallback is true, try to find a parent folder that is a mountpoint. If its false, only the exact matches are checked for mountpoints.
func (l *CheckDrivesize) getRequiredDrives(drives []string, parentFallback bool) (requiredDisks map[string]map[string]string, err error) {
	// create map of required disks/volumes with "drive_or_id" as primary key
	requiredDisks = map[string]map[string]string{}

	for _, drive := range drives {
		switch drive {
		case "*", "all", "all-drives":
			err := l.setDisks(requiredDisks)
			if err != nil {
				return nil, err
			}
		case "all-volumes":
			// nothing appropriate on linux
		default:
			l.hasCustomPath = true
			err := l.setCustomPath(drive, requiredDisks, parentFallback)
			if err != nil {
				return nil, err
			}
		}
	}

	return requiredDisks, nil
}

// setDisks fills the requiredDisks map with all available disks/partitions
func (l *CheckDrivesize) setDisks(requiredDisks map[string]map[string]string) (err error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return fmt.Errorf("disk partitions failed: %s", err.Error())
	}
	for _, partition := range partitions {
		drive := partition.Mountpoint
		entry, ok := requiredDisks[drive]
		if !ok {
			entry = make(map[string]string)
		}

		entry["drive"] = drive
		entry["drive_or_id"] = drive
		entry["drive_or_name"] = drive
		entry["fstype"] = partition.Fstype
		requiredDisks[drive] = entry
	}

	return nil
}

func (l *CheckDrivesize) setCustomPath(path string, requiredDisks map[string]map[string]string, parentFallback bool) (err error) {
	// make sure path exists
	_, err = os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		log.Debugf("%s: %s", path, err.Error())

		return &PartitionNotFoundError{Path: path, err: err}
	}

	// try to find closest matching mount
	availMounts := map[string]map[string]string{}
	err = l.setDisks(availMounts)
	if err != nil {
		return err
	}

	var match *map[string]string
	for i := range availMounts {
		vol := availMounts[i]
		if parentFallback && vol["drive"] != "" && strings.HasPrefix(path, vol["drive"]) {
			// try to find the longest matching parent path, that is a mountpoint
			if match == nil || len((*match)["drive"]) < len(vol["drive"]) {
				match = &vol
			}
		}
		// direct match, no need to search further
		if path == vol["drive"] {
			match = &vol

			break
		}
	}

	if match != nil {
		requiredDisks[path] = utils.CloneStringMap(*match)
		requiredDisks[path]["drive"] = path

		return nil
	}

	// add anyway to generate an error later with more default values filled in
	entry := l.driveEntry(path)
	entry["_error"] = (&PartitionNotMountedError{
		Path: path, err: fmt.Errorf("path :%s does exist, but could not match it to a drive. its likely that the partition is not mounted", path),
	}).Error()
	requiredDisks[path] = entry

	return nil
}

func (l *CheckDrivesize) addDiskDetails(ctx context.Context, check *CheckData, drive map[string]string, magic float64) {
	// check filter before querying disk
	if !check.MatchMapCondition(check.filter, drive, true) {
		return
	}

	// set some defaults
	drive["id"] = ""
	drive["name"] = ""
	drive["mounted"] = "0"

	timeoutContext, cancel := context.WithTimeout(ctx, DiskDetailsTimeout)
	defer cancel()

	usage, err := disk.UsageWithContext(timeoutContext, drive["drive_or_id"])
	if err != nil {
		drive["_error"] = fmt.Sprintf("failed to find disk partition %s: %s", drive["drive_or_id"], err.Error())
		usage = &disk.UsageStat{}
	} else {
		drive["mounted"] = "1"
	}

	l.addDriveSizeDetails(check, drive, usage, magic)
}
