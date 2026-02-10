package snclient

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"
)

const (
	// DefaultSystemMetricsMeasureInterval sets the ticker measuring the CPU counter
	DefaultSystemMetricsMeasureInterval = 1 * time.Second
)

// DefaultSystemTaskConfig sets defaults for windows and unix system task
var DefaultSystemTaskConfig = ConfigData{
	"default buffer length": "15m",
	"device filter":         "^veth",
	"metrics interval":      "5s",
}

// initialization function first discovers partitions
// depending on their type, corresponding device of that partition is added
// non-physical drives are not added to IO counters
var PartitionDevicesToWatch []string

func init() {
	// gopsutil on Linux seems to be reading /proc/partitions and then adding more info according to /sys/class/block/<device>/*
	partitions, err := disk.Partitions(true)

	partitionTypesToExclude := defaultExcludedFsTypes()

	if err == nil {
		for _, partition := range partitions {
			if partition.Device == "none" {
				continue
			}

			if slices.Contains(partitionTypesToExclude, partition.Fstype) {
				continue
			}

			PartitionDevicesToWatch = append(PartitionDevicesToWatch, partition.Device)
		}
	}
}

// This function determines if counters
func DiskEligibleForWatch(diskName string) bool {
	// partitionDevices were calculated in init() function

	// while diskName comes from gopsutil disk.IoCounters()
	// On linux it seems to be reading /proc/diskstats
	// Has entries that look like this:
	// "nvme0n1p2"

	diskEligible := false

	for _, partitionDevice := range PartitionDevicesToWatch {
		if strings.Contains(partitionDevice, diskName) {
			diskEligible = true

			break
		}
	}

	return diskEligible
}

type CheckSystemHandler struct {
	noCopy noCopy

	stopChannel chan bool
	snc         *Agent

	bufferLength    time.Duration
	metricsInterval time.Duration
	deviceFilter    []regexp.Regexp
}

func NewCheckSystemHandler() Module {
	return &CheckSystemHandler{}
}

func (c *CheckSystemHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *AgentRunSet) error {
	c.snc = snc
	c.stopChannel = make(chan bool)

	bufferLength, _, err := section.GetDuration("default buffer length")
	if err != nil {
		return fmt.Errorf("default buffer length: %s", err.Error())
	}
	c.bufferLength = time.Duration(bufferLength) * time.Second

	metricsInterval, _, err := section.GetDuration("metrics interval")
	if err != nil {
		return fmt.Errorf("metrics interval: %s", err.Error())
	}
	if metricsInterval <= 0 {
		metricsInterval = DefaultSystemMetricsMeasureInterval.Seconds()
	}
	c.metricsInterval = time.Duration(metricsInterval) * time.Second

	deviceFilter, ok, err := section.GetRegexp("device filter")
	if err != nil {
		return fmt.Errorf("device filter: %s", err.Error())
	}
	if ok && deviceFilter != nil {
		c.deviceFilter = []regexp.Regexp{*deviceFilter}
	}

	// create counter
	c.update(true)

	return nil
}

func (c *CheckSystemHandler) Start() error {
	go c.mainLoop()

	return nil
}

func (c *CheckSystemHandler) Stop() {
	close(c.stopChannel)
}

func (c *CheckSystemHandler) mainLoop() {
	ticker := time.NewTicker(c.metricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChannel:
			log.Tracef("stopping CheckSystem mainLoop")

			return
		case <-ticker.C:
			c.update(false)

			continue
		}
	}
}

func (c *CheckSystemHandler) update(create bool) {
	data, times, netdata, err := c.fetch()
	if err != nil {
		log.Warnf("[CheckSystem] reading cpu info failed: %s", err.Error())

		return
	}

	if create {
		for key := range data {
			c.snc.counterCreate("cpu", key, c.bufferLength, c.metricsInterval)
		}
		c.snc.counterCreate("cpuinfo", "info", c.bufferLength, c.metricsInterval)
	}

	for key, val := range data {
		c.snc.Counter.Set("cpu", key, val)
	}
	c.snc.Counter.Set("cpuinfo", "info", times)

	// add interface traffic data
	for key, val := range netdata {
		skipped := false
		for i := range c.deviceFilter {
			if c.deviceFilter[i].MatchString(key) {
				skipped = true

				break
			}
		}
		if skipped {
			log.Tracef("skipped network device: %s", key)

			continue
		}
		if c.snc.Counter.Get("net", key) == nil {
			c.snc.counterCreate("net", key, c.bufferLength, c.metricsInterval)
		}
		c.snc.Counter.Set("net", key, val)
	}

	// remove interface not updated within the bufferLength
	trimData := time.Now().Add(-c.bufferLength).UnixMilli()
	for _, key := range c.snc.Counter.Keys("net") {
		last := c.snc.Counter.Get("net", key).GetLast()
		if last.UnixMilli < trimData {
			log.Tracef("removed old net device: %s (last update: %s)", key, time.UnixMilli(last.UnixMilli).String())
			c.snc.Counter.Delete("net", key)
		}
	}

	if runtime.GOOS == "linux" {
		c.addLinuxKernelStats(create)
	}

	// Windows and Non-Windows have their own definitions for this
	c.addDiskStats(create)

	// Windows and Non-Windows have their own defintions for this
	c.addMemoryStats(create)
}

func (c *CheckSystemHandler) fetch() (data map[string]float64, cputimes *cpuinfo.TimesStat, netdata map[string]float64, err error) {
	data = map[string]float64{}

	info, err := cpuinfo.Percent(0, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}

	total := float64(0)
	for i, d := range info {
		data[fmt.Sprintf("core%d", i)] = d
		total += d
	}
	data["total"] = 0
	if len(info) > 0 {
		data["total"] = total / float64(len(info))
	}

	times, err := cpuinfo.Times(false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}

	netdata = map[string]float64{}
	IOList, err := net.IOCounters(true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("net.IOCounters failed: %s", err.Error())
	}

	for intnr, int := range IOList {
		netdata[int.Name+"_recv"] = float64(IOList[intnr].BytesRecv)
		netdata[int.Name+"_sent"] = float64(IOList[intnr].BytesSent)
	}

	return data, &times[0], netdata, nil
}

func (c *CheckSystemHandler) addLinuxKernelStats(create bool) {
	if create {
		c.snc.counterCreate("kernel", "ctxt", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("kernel", "processes", c.bufferLength, c.metricsInterval)
	}

	// psutil has cpu_stats() function which gives the context switch count
	// gopsutil does not have it impelmented yet

	statFile, err := os.Open("/proc/stat")
	if err != nil {
		return
	}
	defer statFile.Close()
	fileScanner := bufio.NewScanner(statFile)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		switch {
		case strings.HasPrefix(line, "ctxt "),
			strings.HasPrefix(line, "processes "):
			row := strings.Fields(line)
			if len(row) < 1 {
				continue
			}
			num := convert.Float64(row[1])
			c.snc.Counter.Set("kernel", row[0], num)
		}
	}
}
