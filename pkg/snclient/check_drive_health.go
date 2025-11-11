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

type SmartctlJsonOutputSmartctl struct {
	Version              []int    `json:"version"`
	PreRelease           bool     `json:"pre_release"`
	SvnVersion           string   `json:"svn_version"`
	PlatformInfo         string   `json:"platform_release"`
	BuildInfo            string   `json:"build_info"`
	Argv                 []string `json:"argv"`
	DriveDatabaseVersion struct {
		String string `json:"string"`
	} `json:"drive_database_version"`
	ExitStatus int `json:"exit_status"`
}

type SmartctlJsonOutputDevice struct {
	Name       string `json:"name"`
	InfoName   string `json:"info_name"`
	DeviceType string `json:"type"`
	Protocol   string `json:"protocol"`
}

type SmartctlJsonOutputLocalTime struct {
	TimeT   uint64 `json:"time_t"`
	Asctime string `json:"asctime"`
}

type SmartctlJsonOutputNvmeNamespaces struct {
	Id               uint64                        `json:"id"`
	Size             SmartctlJsonOutputBlocksBytes `json:"size"`
	Capacity         SmartctlJsonOutputBlocksBytes `json:"capacity"`
	Utilization      SmartctlJsonOutputBlocksBytes `json:"utilization"`
	FormattedLbaSize uint64                        `json:"formatted_lba_base"`
	Eui64            struct {
		Oui   uint64 `json:"oui"`
		ExtId uint64 `json:"ext_id"`
	} `json:"eui64"`
}

type SmartctlJsonOutputBlocksBytes struct {
	Blocks uint64 `json:"id"`
	Bytes  uint64 `json:"bytes"`
}

type SmartctlJsonOutputValueString struct {
	Value  uint64 `json:"value"`
	String string `json:"string"`
}

type SmartctlJsonOutputSmartStatus struct {
	Passed bool `json:"passed"`
	Nvme   *struct {
		Value uint64 `json:"value"`
	} `json:"nvme"`
}

type SmarctlJsonOutputNvmeSmartHealthInformationLog struct {
	CriticalWarning         uint64   `json:"critical_warning"`
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
	SelfTestCode   SmartctlJsonOutputValueString `json:"self_test_code"`
	SelfTestResult SmartctlJsonOutputValueString `json:"self_test_result"`
	PowerOnHours   uint64                        `json:"power_on_hours"`
}

type SmartctlJsonOutputNvmeSelfTestLog struct {
	CurrentSelfTestOperation SmartctlJsonOutputValueString                 `json:"current_self_test_operation"`
	Table                    []SmartctlJsonOutputNvmeSelfTestLogTableEntry `json:"table"`
}

type SmartctlJsonOutputAtaSmartSelfTestLog struct {
	Extended struct {
		Revision uint64 `json:"revision"`
		Sectors  uint64 `json:"sectors"`
		Count    uint64 `json:"count"`
		Table    []struct {
			Type   SmartctlJsonOutputValueString `json:"type"`
			Status struct {
				Value  uint64 `json:"value"`
				String string `json:"string"`
				Passed bool   `json:"passed"`
			}
			LifetimeHours uint64 `json:"lifetime_hours"`
		} `json:"table"`
	}
	Standard struct {
		Revision uint64 `json:"revision"`
		Count    uint64 `json:"count"`
	}
}

type SmartctlJsonOutputAtaSmartSelectiveSelfTestlog struct {
	Revision uint64 `json:"revision"`
	Table    []struct {
		LbaMin uint64 `json:"lba_min"`
		LbaMax uint64 `json:"lba_max"`
		Status SmartctlJsonOutputValueString
	} `json:"table"`
	Flags struct {
		Value               uint64 `json:"value"`
		ReminderScanEnabled bool   `json:"reminder_scan_enabled"`
	}
	PowerUPScanResumeMinutes uint64 `json:"power_up_scan_resume_minutes"`
}

type SmartctlJsonOutputAtaSmartData struct {
	OfflineDataCollection struct {
		Status            SmartctlJsonOutputValueString `json:"status"`
		CompletionSeconds uint64                        `json:"completion_seconds"`
	} `json:"offline_data_collection"`
	SelfTest struct {
		Status         SmartctlJsonOutputValueString `json:"status"`
		PollingMinutes struct {
			Short    uint64 `json:"short"`
			Extended uint64 `json:"extended"`
		} `json:"polling_minutes"`
	} `json:"self_test"`
	Capabilities struct {
		Values                        []uint64 `json:"values"`
		ExecOfflineImmediateSupported bool     `json:"exec_online_immediate_supported"`
		OfflineIsAbortedUponNewCmd    bool     `json:"offline_is_aborted_upon_new_cmd"`
		OfflineSurfaceScanSupported   bool     `json:"offline_surface_scan_supported"`
		SelfTestsSupported            bool     `json:"self_tests_supported"`
		ConveyanceSelfTestSupported   bool     `json:"conveyance_self_test_supported"`
		SelectiveSelfTestSupported    bool     `json:"selective_self_test_supported"`
		AttributeAutosaveEnabled      bool     `json:"attribute_autosave_enabled"`
		ErrorLoggingSupoorted         bool     `json:"error_logging_supported"`
		GpLogginSupported             bool     `json:"gp_logging_supported"`
	} `json:"capabilities"`
}

type SmartctlJsonOutputAtaAttribute struct {
	Id         uint64 `json:"id"`
	Name       string `json:"name"`
	Value      uint64 `json:"value"`
	Worst      uint64 `json:"worst"`
	Thresh     uint64 `json:"thresh"`
	WhenFailed string `json:"when_failed"`
	Flags      struct {
		Value         uint64 `json:"value"`
		String        string `json:"string"`
		Prefailure    bool   `json:"prefailure"`
		UpdatedOnline bool   `json:"updated_online"`
		Performance   bool   `json:"performance"`
		ErrorRate     bool   `json:"error_rate"`
		EventCount    bool   `json:"event_count"`
		AutoKeep      bool   `json:"auto_keep"`
	} `json:"flags"`
	Raw SmartctlJsonOutputValueString `json:"raw"`
}

// smartctl --scan-open json
type SmartctlJsonOutputScanOpen struct {
	JsonFormatVersion []uint64                   `json:"json_format_version"`
	Smartctl          SmartctlJsonOutputSmartctl `json:"smartctl"`
	Devices           []SmartctlJsonOutputDevice `json:"devices"`
}

// smartctl --json --test short /dev/nvme0
type SmartctlJsonOutputStartScan struct {
	JsonFormatVersion []uint64                    `json:"json_format_version"`
	Smartctl          SmartctlJsonOutputSmartctl  `json:"smartctl"`
	Devices           []SmartctlJsonOutputDevice  `json:"devices"`
	LocalTime         SmartctlJsonOutputLocalTime `json:"local_time"`
}

// smartctl --json --xall /dev/sda
type SmartctlJsonOutputXall struct {
	// Always present
	JsonFormatVersion []uint64 `json:"json_format_version"`
	// Always present
	Smartctl SmartctlJsonOutputSmartctl `json:"smartctl"`
	// Always present
	LocalTime SmartctlJsonOutputLocalTime `json:"local_time"`
	// Might be missing if you type the device name wrong
	Device SmartctlJsonOutputDevice `json:"device"`
	// Not always reported
	ScsiVendor string `json:"scsi_vendor"`
	// Not always reported
	ScsiProduct string `json:"scsi_product"`
	// Not always reported
	ScsiRevision string `json:"scsi_revision"`
	// Not always reported
	ScsiVersion string `json:"scsi_version"`
	// Not always reported
	ModelFamily string `json:"model_family"`
	// Not always reported
	ModelName string `json:"model_name"`
	// Seems to be always present???
	SerialNumber string `json:"serial_number"`
	Wwn          struct {
		Naa uint64 `json:"naa"`
		Oui uint64 `json:"oui"`
		Id  uint64 `json:"id"`
	} `json:"wwn"`
	FirmwareVersion string `json:"firmware_version"`
	NvmePciVendor   struct {
		Id          uint64 `json:"id"`
		SubsystemId uint64 `json:"subsystem_id"`
	} `json:"nvme_pci_vendor"`
	NvmeIeeeOuiIdentifier  uint64                             `json:"nvme_ieee_oui_identifier"`
	NvmeControllerId       uint64                             `json:"nvme_controller_id"`
	NvmeVersion            SmartctlJsonOutputValueString      `json:"nvme_version"`
	NvmeNumberOfNamespaces uint64                             `json:"nvme_number_of_namespaces"`
	NvmeNamespaces         []SmartctlJsonOutputNvmeNamespaces `json:"nvme_namespaces"`
	UserCapacity           SmartctlJsonOutputBlocksBytes      `json:"user_capacity"`
	LogicalBlockSize       uint64                             `json:"logical_block_size"`
	PhysicalBlockSize      uint64                             `json:"physical_block_size"`
	RotationRate           uint64                             `json:"rotation_rate"`
	FormFactor             struct {
		AtaValue uint64 `json:"ata_value"`
		Name     string `json:"name"`
	} `json:"form_factor"`
	Trim struct {
		Supported bool `json:"supported"`
	} `json:"trim"`
	InSmartctlDatabase bool `json:"in_smartctl_database"`
	AtaVersion         struct {
		String     string `json:"string"`
		MajorValue uint64 `json:"major_value"`
		MinorValue uint64 `json:"minor_value"`
	} `json:"ata_version"`
	SataVersion struct {
		String string `json:"string"`
		Value  uint64 `json:"value"`
	} `json:"sata_version"`
	InterfaceSpeed struct {
		Max struct {
			SataValue      uint64 `json:"sata_value"`
			String         string `json:"string"`
			UnitsPerSecond uint64 `json:"units_per_second"`
			BitsPerUnit    uint64 `json:"bits_per_unit"`
		} `json:"max"`
		Current struct {
			SataValue      uint64 `json:"sata_value"`
			String         string `json:"string"`
			UnitsPerSecond uint64 `json:"units_per_second"`
			BitsPerUnit    uint64 `json:"bits_per_unit"`
		} `json:"current"`
	} `json:"interface_speed"`
	ReadLookahead struct {
		Enabled bool `json:"enabled"`
	} `json:"read_lookahead"`
	WriteCache struct {
		Enabled bool `json:"write_cache"`
	} `json:"write_cache"`
	AtaSecurity struct {
		State            uint64 `json:"state"`
		String           string `json:"string"`
		Enabled          bool   `json:"enabled"`
		Frozen           bool   `json:"frozen"`
		MasterPasswordId uint64 `json:"master_password_id"`
	} `json:"ata_security"`
	SmartSupport struct {
		Available bool `json:"available"`
		Enabled   bool `json:"enabled"`
	} `json:"smart_support"`
	SmartStatuts                  *SmartctlJsonOutputSmartStatus                 `json:"smart_status"`
	NvmeSmartHealthInformationLog SmarctlJsonOutputNvmeSmartHealthInformationLog `json:"nvme_smart_health_information_log"`
	Temperature                   struct {
		Current       int64  `json:"current"`
		DriveTrip     *int64 `json:"drive_trip"`
		PowerCycleMin *int64 `json:"power_cycle_min"`
		PowerCycleMax *int64 `json:"power_cycle_max"`
		LifetimeMin   *int64 `json:"lifetime_min"`
		LifetimeMax   *int64 `json:"lifetime_max"`
		OpLimitMin    *int64 `json:"op_limit_min"`
		OpLimitMax    *int64 `json:"op_limit_max"`
		LimitMin      *int64 `json:"limit_min"`
		LimitMax      *int64 `json:"limit_max"`
	} `json:"temperature"`
	PowerCycleCount uint64 `json:"power_cycle_count"`
	PowerOnTime     struct {
		Hours   uint64 `json:"hours"`
		Minutes uint64 `json:"minutes"`
	} `json:"power_on_time"`
	NvmeErrorInformationLog struct {
		Size   uint64 `json:"size"`
		Read   uint64 `json:"read"`
		Unread uint64 `json:"unread"`
	} `json:"nvme_error_information_log"`
	NvmeSelfTestLog              SmartctlJsonOutputNvmeSelfTestLog              `json:"nvme_self_test_log"`
	AtaSmartSelfTestLog          SmartctlJsonOutputAtaSmartSelfTestLog          `json:"ata_smart_self_test_log"`
	AtaSmartSelectiveSelfTestLog SmartctlJsonOutputAtaSmartSelectiveSelfTestlog `json:"ata_smart_selective_self_test_log"`
	AtaSmartData                 SmartctlJsonOutputAtaSmartData                 `json:"ata_smart_data"`
	AtaSctCapabilities           struct {
		Value                         uint64 `json:"value"`
		ErrorRecoveryControlSupported bool   `json:"error_recovery_control_supported"`
		FeatureControlSupported       bool   `json:"feature_control_supported"`
		DataTableSupported            bool   `json:"data_table_supported"`
	} `json:"ata_sct_capabilities"`
	AtaSmartErrorLog struct {
		Summary struct {
			Revision uint64 `json:"revision"`
			Count    uint64 `json:"count"`
		} `json:"summary"`
	} `json:"ata_smart_error_log"`
	AtaSmartAttributes struct {
		Revision uint64                           `json:"revision"`
		Table    []SmartctlJsonOutputAtaAttribute `json:"table"`
	}
}
