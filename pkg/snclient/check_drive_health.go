package snclient

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/goccy/go-json"
)

func init() {
	AvailableChecks["check_drive_health"] = CheckEntry{"check_drive_health", NewCheckDriveHealth}
}

type CheckDriveHealth struct {
	// The logical device name of the drive e.g /dev/sda /dev/nvme0
	drive_filter CommaStringList
	// Test to run. Should be one of these values: 'offline' , 'short' , 'long' , 'conveyance' , 'select'
	test_type string
	// Logical block address to start the test on. Has to be a number. Required if the test type is 'select'
	test_start_lba uint64
	// End block of the test. Can also be specified as 'max' or a number. Required if the test type is 'select'
	test_end_lba string
}

func NewCheckDriveHealth() CheckHandler {
	return &CheckDriveHealth{
		drive_filter:   make([]string, 1),
		test_type:      "offline",
		test_start_lba: 0,
		test_end_lba:   "max",
	}
}

func (checkDriveHealth *CheckDriveHealth) Build() *CheckData {
	return &CheckData{
		name:        "check_drive_health",
		description: "Checks the interrupts on CPUs",
		implemented: Linux,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"drive_filter": {
				value:       &checkDriveHealth.drive_filter,
				description: "Drives to check health for. Give iti as a comma seperated list of logical device names e.g '/dev/sda,'/dev/nvme0' . Leaving it empty will check all drives which report a SMART status.",
			},
			"test_type": {
				value:       &checkDriveHealth.test_type,
				description: "SMART test type to perform for checking the health of the drives. ",
			},
			"test_start_lba": {
				value:       &checkDriveHealth.test_start_lba,
				description: "Logical block address to start the test on. Has to be specified as a number. Needed if the test type is 'select'",
			},
			"test_end_lba": {
				value:       &checkDriveHealth.test_end_lba,
				description: "Logical block address to end the test on, inclusive. Can be specified as a number or as 'max' to select the last block on the disk. Required if the test type is 'select'",
			},
		},
		detailSyntax: "%(drive_name)|%(test_type) test from $(test_start_lba)-%(test_end_lba)",
		okSyntax:     "%(status) - All %(count) drives are ok",
		topSyntax:    "%(status) - %(problem_count)/%(count) drives , %(problem_list)",
		emptySyntax:  "Failed to find any drives matching this filter",
		emptyState:   CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "test_result", description: "The result of the test. Takes possible outputs: \"PASS\" , \"FAIL\" "},
			{name: "test_details", description: "The details of the test given by smartctl"},
			{name: "test_type", description: "Hardware interrupt lines of the CPU. These are assigned between CPU and other devices by the system firmware, and get an number"},
			{name: "test_drive", description: "The drive the test was performed on"},
			{name: "test_start_lba", description: "Logical block address the test was started on. Required if test type is 'select' "},
			{name: "test_end_lba", description: "Logical block address the test was ended on. Required if the test type is 'select' "},
		},
		exampleDefault: `
Perform a short test on drive with the name nvme0
		check_interrupts drive_filter="nvme0" test_type="short"
		`,
	}
}

func (checkDriveHealth *CheckDriveHealth) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {

	scan_output, err := SmartctlScanOpen()
	if err != nil {
		return nil, fmt.Errorf("error when discovering smartctl devices : %s", err.Error())
	}

	var drive_filter []string
	if drive_filter_argument, ok := check.args["drive_filter"]; ok {
		if drive_filter_csl_ptr, ok := drive_filter_argument.value.(*CommaStringList); ok {
			csl_value := *drive_filter_csl_ptr
			drive_filter = []string(csl_value)
		} else {
			return nil, fmt.Errorf("unexpected type for drive_filter_argument.value , expected *CommaStringList")
		}
	}

	entry := map[string]string{}
	entry["test_type"] = checkDriveHealth.test_type
	entry["test_start_lba"] = strconv.FormatUint(checkDriveHealth.test_start_lba, 10)
	entry["test_end_lba"] = checkDriveHealth.test_end_lba

	var valid_test_types []string = []string{"short", "long", "conveyance", "select"}
	if !slices.Contains(valid_test_types, entry["test_type"]) {
		return nil, fmt.Errorf("unexpected test type to perform: %s , valid test types are: %s", entry["test_type"], strings.Join(valid_test_types, ","))
	}

	var test_string string
	if entry["test_type"] == "select" {
		if entry["test_start_lba"] == "" || entry["test_end_lba"] == "" {
			return nil, fmt.Errorf("if the test type is 'select', the test_start_lba and test_end_lba arguments have to be specified")
		} else {
			test_string = entry["test_type"] + "," + entry["test_start_lba"] + "-" + entry["test_end_lba"]
		}
	} else {
		test_string = entry["test_type"]
	}

	for _, drive := range scan_output.Devices {
		if slices.Contains(drive_filter, drive.Name) {
			SmartctlCompleteScan(drive.Name, test_string)
		}
	}

	check.listData = append(check.listData, entry)

	needCpu := check.HasThreshold("cpu")
	for _, data := range check.listData {
		if needCpu {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "cpu",
					Name:          fmt.Sprintf("id: %s ; name: %s ; cpu: %s ; interrupt_count: %s", data["interrupt_number"], data["interrupt_name"], data["cpu"], data["interrupt_count"]),
					Value:         convert.UInt32(data["cpu"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}
	}

	return check.Finalize()

}

type SmartctlJsonOutputJsonFormatVersion []int

type SmartctlJsonOutputSmartctl struct {
	Version      []int    `json:"version"`
	PreRelease   bool     `json:"pre_release"`
	SvnVersion   string   `json:"svn_version"`
	PlatformInfo string   `json:"platform_release"`
	BuildInfo    string   `json:"build_info"`
	Argv         []string `json:"argv"`
	ExitStatus   int      `json:"exit_status"`
}

type SmartctlJsonOutputDevice struct {
	Name       string `json:"name"`
	InfoName   string `json:"info_name"`
	DeviceType string `json:"type"`
	Protocol   string `json:"protocol"`
}

type SmartctlJsonOutputLocalTime struct {
	TimeT   string `json:"time_t"`
	Asctime string `json:"asctime"`
}

// smartctl --scan-open json
type SmartctlJsonOutputScanOpen struct {
	JsonFormatVersion SmartctlJsonOutputJsonFormatVersion `json:"json_format_version"`
	Smartctl          SmartctlJsonOutputSmartctl          `json:"smartctl"`
	Devices           []SmartctlJsonOutputDevice          `json:"devices"`
}

// smartctl --json --test short /dev/nvme0
type SmartctlJsonOutputStartScan struct {
	JsonFormatVersion SmartctlJsonOutputJsonFormatVersion `json:"json_format_version"`
	Smartctl          SmartctlJsonOutputSmartctl          `json:"smartctl"`
	Devices           []SmartctlJsonOutputDevice          `json:"devices"`
	LocalTime         SmartctlJsonOutputLocalTime         `json:"local_time"`
}

type SmartctlJsonOutputNvmePciVendor struct {
	Id          uint64 `json:"id"`
	SubsystemId uint64 `json:"subsystem_id"`
}

type SmartctlJsonOutputNvmeVersion struct {
	StringVersion string `json:"string"`
	Value         uint64 `json:"value"`
}

type SmartctlJsonOutputNvmeNamespaces struct {
	Id               uint64                                `json:"id"`
	Size             SmartctlJsonOutputNvmeNamespacesBlock `json:"size"`
	Capacity         SmartctlJsonOutputNvmeNamespacesBlock `json:"capacity"`
	Utilization      SmartctlJsonOutputNvmeNamespacesBlock `json:"utilization"`
	FormattedLbaSize uint64                                `json:"formatted_lba_base"`
	Eui64            SmartctlJsonOutputNvmeNamespacesEui64 `json:"eui64"`
}

type SmartctlJsonOutputNvmeNamespacesBlock struct {
	Blocks uint64 `json:"id"`
	Bytes  uint64 `json:"bytes"`
}

type SmartctlJsonOutputNvmeNamespacesEui64 struct {
	Oui   uint64 `json:"oui"`
	ExtId uint64 `json:"ext_id"`
}

type SmartctlJsonOutputSmartSupport struct {
	Available bool `json:"available"`
	Enabled   bool `json:"enabled"`
}

type SmartctlJsonOutputSmartStatus struct {
	Passed bool `json:"passed"`
	Nvme   struct {
		Value uint `json:"value"`
	} `json:"nvme"`
}

type SmarctlJsonOutputNvmeSmartHealthInformationLog struct {
	CriticalWarning         bool     `json:"critical_warning"`
	Temperature             uint64   `json:"temperature"`
	AvailableSpare          uint64   `json:"available_spare"`
	AvailableSpareThreshold uint64   `json:"available_spare_threshold"`
	PercentageUsed          uint64   `json:"percentage_used"`
	DataUnitsRead           uint64   `json:"data_units_read"`
	DataUnitsWritten        uint64   `json:"data_units_written"`
	HostReads               uint64   `json:"host_reads"`
	HostWrites              uint64   `json:"host_writes"`
	ControllerBusyTime      uint64   `json:"controller_busy_type"`
	PowerCycles             uint64   `json:"power_cycles"`
	PowerOnHours            uint64   `json:"power_on_hours"`
	UnsafeShutdowns         uint64   `json:"unsafe_shutdowns"`
	MediaErrors             uint64   `json:"media_errors"`
	NumErrLogEntries        uint64   `json:"num_err_log_entries"`
	WarningTempTime         uint64   `json:"warning_temp_time"`
	CriticalCompTime        uint64   `json:"critical_comp_time"`
	TemperatureSensors      []uint64 `json:"temperature_sensors"`
}

type SmartctlJsonOutputNvmeSelfTestLogTableEntry struct {
	SelfTestCode struct {
		Value        uint64 `json:"value"`
		StatusString string `json:"string"`
	}
	SelfTestResult struct {
		Value        uint64 `json:"value"`
		StatusString string `json:"string"`
	}
	PowerOnHours uint64 `json:"power_on_hours"`
}

type SmartctlJsonOutputNvmeSelfTestLog struct {
	CurrentSelfTestOperation struct {
		Value        uint64 `json:"value"`
		StatusString string `json:"string"`
	} `json:"current_self_test_operation"`
	Table []SmartctlJsonOutputNvmeSelfTestLogTableEntry `json:"table"`
}

type SmartctlJsonOutputXall struct {
	JsonFormatVersion             SmartctlJsonOutputJsonFormatVersion            `json:"json_format_version"`
	Smartctl                      SmartctlJsonOutputSmartctl                     `json:"smartctl"`
	LocalTime                     SmartctlJsonOutputLocalTime                    `json:"local_time"`
	Device                        SmartctlJsonOutputDevice                       `json:"device"`
	ModelName                     string                                         `json:"model_name"`
	SerialNumber                  string                                         `json:"serial_number"`
	FirmwareVersion               string                                         `json:"firmware_version"`
	NvmePciVendor                 SmartctlJsonOutputNvmePciVendor                `json:"nvme_pci_vendor"`
	NvmeIeeeOuiIdentifier         string                                         `json:"nvme_ieee_oui_identifier"`
	NvmeControllerId              uint64                                         `json:"nvme_controller_id"`
	NvmeVersion                   SmartctlJsonOutputNvmeVersion                  `json:"nvme_version"`
	NvmeNumberOfNamespaces        uint64                                         `json:"nvme_number_of_namespaces"`
	NvmeNamespaces                []SmartctlJsonOutputNvmeNamespaces             `json:"nvme_namespaces"`
	UserCapacity                  SmartctlJsonOutputNvmeNamespacesBlock          `json:"user_capacity"`
	LogicalBlockSize              uint64                                         `json:"logical_block_size"`
	SmartSupport                  SmartctlJsonOutputSmartSupport                 `json:"smart_support"`
	NvmeSmartHealthInformationLog SmarctlJsonOutputNvmeSmartHealthInformationLog `json:"nvme_smart_health_information_log"`
	Temperature                   struct {
		Current uint64 `json:"current"`
	} `json:"temperature"`
	PowerCycleCount uint64 `json:"power_cycle_count"`
	PowerOnTime     struct {
		Hours uint64 `json:"hours"`
	} `json:"power_on_time"`
	NvmeErrorInformationLog struct {
		Size   uint64 `json:"size"`
		Read   uint64 `json:"read"`
		Unread uint64 `json:"unread"`
	}
	NvmeSelfTestLog SmartctlJsonOutputNvmeSelfTestLog `json:"nvme_self_test_log"`
}

func SmartctlCompleteScan(device string, test_string string) error {
	smartctl_executable, err := exec.LookPath("smartctl")
	if err != nil {
		return fmt.Errorf("could not find smartctl executable, are you running as a priviledged user: %s", err.Error())
	}

	cmd := exec.Command(smartctl_executable, "--json", "--test", test_string, device)

	stdout, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("running command for starting the scan: '%s' failed with exit code: '%d' and stderr: '%s'", strings.Join(cmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}
		return fmt.Errorf("running command for starting the scan: '%s' failed with error: '%s' ", strings.Join(cmd.Args, " "), err.Error())
	}

	var output SmartctlJsonOutputStartScan
	if err := json.Unmarshal(stdout, &output); err != nil {
		return fmt.Errorf("could not parse command output for starting the scan: '%s' ", strings.Join(cmd.Args, " "))
	}

	if output.Smartctl.ExitStatus != 0 {
		return fmt.Errorf("there seems to be an error with starting the scan: '%s' ", strings.Join(cmd.Args, " "))
	}

	return nil
}

func SmartctlScanOpen() (*SmartctlJsonOutputScanOpen, error) {
	smartctl_executable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable, are you running as a priviledged user: %s", err.Error())
	}

	cmd := exec.Command(smartctl_executable, "--scan-open", "--json")

	stdout, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Output runs the command and returns its standard output. Any returned error will usually be of type [*ExitError]. If c.Stderr was nil and the returned error is of type [*ExitError], Output populates the Stderr field of the returned error.
			return nil, fmt.Errorf("running command: '%s' failed with exit code: '%d' and stderr: '%s'", strings.Join(cmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}
		return nil, fmt.Errorf("running command: '%s' failed with an error: '%s'", strings.Join(cmd.Args, " "), err.Error())
	}

	var output SmartctlJsonOutputScanOpen
	if err := json.Unmarshal(stdout, &output); err != nil {
		return nil, fmt.Errorf("could not parse command output: '%s' ", strings.Join(cmd.Args, " "))
	}

	return &output, nil
}
