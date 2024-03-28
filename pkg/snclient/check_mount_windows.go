package snclient

import (
	"context"
	"os"
	"strings"

	"golang.org/x/exp/slices"
)

// getVolumes retrieves volumes and their details, excluding specified partitions, and returns a list of drives and any potential errors.
//
// ctx - The context of the operation.
// partitionMap - A map of mounted partitions to exclude from the retrieval.
// []map[string]string, error - Returns a list of drives and any potential errors.
func (l *CheckMount) getVolumes(ctx context.Context, partitionMap map[string]bool) (drives []map[string]string, err error) {
	driveSize := &CheckDrivesize{}
	volumes := map[string]map[string]string{}
	driveSize.setVolumes(volumes)
	excludes := defaultExcludedFsTypes()

	for i := range volumes {
		partition := volumes[i]
		driveSize.addDiskDetails(ctx, nil, partition, 1)
		if mounted, ok := partition["mounted"]; ok {
			if mounted == "0" {
				continue
			}
		}
		mountPoint := strings.TrimSuffix(partition["drive_or_id"], string(os.PathSeparator))
		if _, ok := partitionMap[mountPoint]; ok {
			continue
		}
		partitionMap[mountPoint] = true
		if l.mountPoint != "" && mountPoint != l.mountPoint {
			log.Tracef("skipped mountpoint: %s - not matching mount argument", mountPoint)

			continue
		}
		// skip internal filesystems
		if slices.Contains(excludes, partition["fstype"]) {
			log.Tracef("skipped mountpoint: %s - fstype %s is excluded", mountPoint, partition["fstype"])

			continue
		}

		entry := map[string]string{
			"mount":   mountPoint,
			"device":  mountPoint,
			"fstype":  partition["fstype"],
			"options": partition["opts"],
			"issues":  "",
		}
		drives = append(drives, entry)
	}

	return drives, nil
}
