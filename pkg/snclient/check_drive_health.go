package snclient

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

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

var valid_test_types []string = []string{"offline", "short", "long", "conveyance", "select"}

func NewCheckDriveHealth() CheckHandler {
	return &CheckDriveHealth{
		drive_filter:   make([]string, 1),
		test_type:      "offline",
		test_start_lba: 0,
		test_end_lba:   "0",
	}
}

func (checkDriveHealth *CheckDriveHealth) Build() *CheckData {
	return &CheckData{
		name:        "check_drive_health",
		description: "Runs a SMART test and reports the test result alongside the smart health status.",
		implemented: Linux,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"drive_filter": {
				value:       &checkDriveHealth.drive_filter,
				description: "Drives to check health for. Give it as a comma seperated list of logical device names e.g '/dev/sda,'/dev/nvme0' . Leaving it empty will check all drives which report a SMART status.",
			},
			"test_type": {
				value:       &checkDriveHealth.test_type,
				description: fmt.Sprintf("SMART test type to perform for checking the health of the drives. Available test types are: '%s' ", strings.Join(valid_test_types, ",")),
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
		// Add a condition that by default reports ok if test_result == "PASSED" && smart_health_status == "PASSED"
		defaultCritical: " test_result != 'PASSED' && smart_health_status != 'PASSED' ",
		// // Add a condition that by default reports warning if test_result == "PASSED" && smart_health_status != "PASSED"
		defaultWarning: " test_result == 'PASSED' && smart_health_status != 'PASSED' ",
		detailSyntax:   "%(drive_name)|%(test_type) test, $(test_start_lba)-%(test_end_lba)",
		okSyntax:       "%(status) - All %(count) drives are ok",
		topSyntax:      "%(status) - %(problem_count)/%(count) drives , %(problem_list)",
		emptySyntax:    "Failed to find any drives matching this filter",
		emptyState:     CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "test_type", description: "Hardware interrupt lines of the CPU. These are assigned between CPU and other devices by the system firmware, and get an number"},
			{name: "test_drive", description: "The drive the test was performed on"},
			{name: "test_result", description: "The result of the test. Takes possible outputs: \"PASSED\" , \"FAILED\" , \"UNKNOWN\" ."},
			{name: "test_details", description: "The details of the test given by smartctl."},
			{name: "test_start_lba", description: "Logical block address the test was started on. Required if test type is 'select' "},
			{name: "test_end_lba", description: "Logical block address the test was ended on. Required if the test type is 'select' "},
			{name: "smart_health_status", description: "SMART overall health self-assesment done by the firmware with the current SMART attributes. It is evaluated independently from the test result, but is just as important. Takes possible values: \"PASSED\" , \"FAILED\" , \"UNKNOWN\" ."},
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

	var wg sync.WaitGroup
	type SmartctlTestResult struct {
		device    SmartctlJsonOutputDevice
		xall_json *SmartctlJsonOutputXall
		err       error
	}
	results_channel := make(chan SmartctlTestResult)

	for _, device := range scan_output.Devices {
		if len(drive_filter) > 0 && !slices.Contains(drive_filter, device.Name) {
			// skip this device
			continue
		}

		wg.Add(1)

		// Define and call an asynchronus function, returns a channel result
		go func(device SmartctlJsonOutputDevice) {
			// This will decrement the waitgroup counter before returning
			defer wg.Done()

			result := SmartctlTestResult{device: device}
			xall_json, xall_json_err := SmartctlXall(device.Name)

			if xall_json_err != nil {
				result.err = fmt.Errorf("error when getting details about the drive: %s , error : %s", device.Name, xall_json_err.Error())
			} else if !xall_json.SmartSupport.Available {
				result.err = fmt.Errorf("device does not support SMART: %s", device.Name)
			} else if xall_json.SmartSupport.Enabled != nil && !*xall_json.SmartSupport.Enabled {
				result.err = fmt.Errorf("device does supports SMART, but does not have it enabled: %s", device.Name)
			} else {
				xall_json, xall_json_err = SmartctlTestAndAwaitCompletion(device.Name, test_string)
				result.xall_json = xall_json
				result.err = xall_json_err
			}

			results_channel <- result

		}(device)

	}

	go func() {
		// will wait until counter reaches 0
		wg.Wait()
		// close the channel afterwards, no more results can be sent there
		close(results_channel)
	}()

	var collectedResults []SmartctlTestResult
	for result := range results_channel {
		collectedResults = append(collectedResults, result)
	}

	// Add results as data points to the check
	for _, result := range collectedResults {
		entry := maps.Clone(entry)
		entry["test_drive"] = result.device.Name

		if result.err != nil {
			return nil, fmt.Errorf("encountered an error while performing the test and awaiting results for device: %s , error: %s", entry["test_drive"], result.err.Error())
		}

		// There are two types of results we can check
		// 1. Test result -> offline, short, long etc.
		// This depends on the disk type, so I am writing a helper function

		entry["test_result"], entry["test_details"] = GetLatestTestResult(result.xall_json)

		// 2. SmartStatus.Passed if it exists.
		// That seems to be the SMART overall-health self-assessment test result

		if result.xall_json.SmartStatuts == nil {
			return nil, fmt.Errorf("smart overall health self-assesment not available on device: %s", entry["test_drive"])
		}

		if result.xall_json.SmartStatuts.Passed {
			entry["smart_health_status"] = "PASSED"
		} else {
			entry["smart_health_status"] = "FAILED"
		}

		check.listData = append(check.listData, entry)
	}

	// for _, data := range check.listData {
	// 	if check.HasThreshold("test_result") {
	// 		check.result.Metrics = append(check.result.Metrics,
	// 			&CheckMetric{
	// 				ThresholdName: "test_result_passed",
	// 				Name:          fmt.Sprintf("drive: %s test_result_passed", data["test_drive"]),
	// 				Value:         (data["test_result"] == "PASSED"),
	// 				Unit:          "",
	// 				// TODO: Fix these perf metrics, dont know how to define them
	// 				Warning:  nil,
	// 				Critical: nil,
	// 				Min:      &Zero,
	// 			},
	// 		)
	// 	}
	// 	if check.HasThreshold("smart_health_status") {
	// 		check.result.Metrics = append(check.result.Metrics,
	// 			&CheckMetric{
	// 				ThresholdName: "smart_health_status_passed",
	// 				Name:          fmt.Sprintf("drive: %s smart_health_status_passed", data["test_drive"]),
	// 				Value:         (data["smart_health_status"] == "PASSED"),
	// 				Unit:          "",
	// 				// TODO: Fix these perf metrics, dont know how to define them
	// 				Warning:  nil,
	// 				Critical: nil,
	// 				Min:      &Zero,
	// 			},
	// 		)
	// 	}
	// }

	return check.Finalize()

}

func GetLatestTestResult(xall_json *SmartctlJsonOutputXall) (result string, details string) {
	switch xall_json.Device.Type {
	case "scsi":
		return "UNKNOWN", fmt.Sprintf("getting the latest test result for drive type %s is not yet implemented", xall_json.Device.Type)
	case "sat":
		// Need to access xall_json.AtaSmartData
		if xall_json.AtaSmartData == nil {
			return "UNKNOWN", fmt.Sprintf("smartctl drive details does not have ata_smart_data, cant get latest test details: %s", xall_json.Device.Name)
		}
		if xall_json.AtaSmartData.SelfTest.Status.Passed == nil {
			return "UNKNOWN", fmt.Sprintf("smartctl drive details does not have ata_smart_data.self_test.status.passed , cant get result the latest completed test: %s", xall_json.Device.Name)
		}
		if *xall_json.AtaSmartData.SelfTest.Status.Passed {
			return "PASSED", xall_json.AtaSmartData.SelfTest.Status.String
		} else {
			return "FAILED", xall_json.AtaSmartData.SelfTest.Status.String
		}
	case "nvme":
		if xall_json.NvmeSelfTestLog == nil {
			return "UNKNOWN", fmt.Sprintf("smarctl drive details does not have nvme_self_test_log, cant get latest test details: %s", xall_json.Device.Name)
		}
		if len(xall_json.NvmeSelfTestLog.Table) == 0 {
			return "UNKNOWN", fmt.Sprintf("smarctl drive details nvme_self_test_log.table is empty, cant get latest test details: %s", xall_json.Device.Name)
		}
		if xall_json.NvmeSelfTestLog.Table[0].SelfTestResult.Value == 0 && xall_json.NvmeSelfTestLog.Table[0].SelfTestResult.String == "Completed without error" {
			return "PASSED", xall_json.NvmeSelfTestLog.Table[0].SelfTestResult.String
		} else {
			return "FAILED", xall_json.NvmeSelfTestLog.Table[0].SelfTestResult.String
		}
	default:
		return "UNKNOWN", fmt.Sprintf("getting the latest test result for drive type %s is not yet implemented", xall_json.Device.Type)
	}
}

func SmartctlTestAndAwaitCompletion(device string, test_string string) (*SmartctlJsonOutputXall, error) {

	// Start a test

	var start_test_json *SmartctlJsonOutputStartTest
	var start_test_err error
	if start_test_json, start_test_err = SmartctlStartTest(device, test_string); start_test_err != nil {
		return nil, fmt.Errorf("error when starting a test: %s", start_test_err.Error())
	}

	device_type := start_test_json.Device.Type
	fmt.Printf("Started test on device: %s witht the device type: %s", start_test_json.Device.Name, start_test_json.Device.Type)

	var xall_json *SmartctlJsonOutputXall
	var xall_json_err error

	// Wait 1 seconds for the test to start
	time.Sleep(time.Second)

	// Busy loop until the test is complete?
busyloop_test_completion:
	for {

		// Get the latest values from device
		if xall_json, xall_json_err = SmartctlXall(device); xall_json_err != nil {
			return nil, fmt.Errorf("error when getting device details: %s", xall_json_err.Error())
		}

		switch device_type {
		case "scsi":
			return nil, fmt.Errorf("testing not yet implemented for type: %s", device_type)
		case "sat":

			if xall_json.AtaSmartData == nil {
				return nil, fmt.Errorf("ata_smart_data is not present xall output")
			}
			if xall_json.AtaSmartData.SelfTest.Status.Value != 0 {
				// the test is presumably still running
				break
			}
			if xall_json.AtaSmartData.SelfTest.Status.RemainingPercent != nil {
				// the test is presumably still running
				break
			}

			break busyloop_test_completion
		case "nvme":
			if xall_json.NvmeSelfTestLog == nil {
				return nil, fmt.Errorf("nvme_self_test_log is not present xall output")
			}
			if xall_json.NvmeSelfTestLog.CurrentSelfTestOperation.Value == 1 {
				// the test is still running
				break
			}

			break busyloop_test_completion
		default:
			return nil, fmt.Errorf("testing not yet implemented for type: %s", device_type)
		}

		// Wait after each check
		time.Sleep(time.Second * 10)
	}

	return xall_json, nil
}

// smartctl --test <test_string> --json <device>
func SmartctlStartTest(device string, test_string string) (*SmartctlJsonOutputStartTest, error) {
	// Find smartctl
	smartctl_executable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable in $PATH: %s , are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	// Start the test . A test can always be started without knowing the device details
	test_start_cmd := exec.Command(smartctl_executable, "--json", "--test", test_string, device)
	test_start_cmd_stdout, err := test_start_cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("running command for starting the scan: '%s' failed with exit code: '%d' and stderr: '%s'", strings.Join(test_start_cmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}
		return nil, fmt.Errorf("running command for starting the scan: '%s' failed with error: '%s' ", strings.Join(test_start_cmd.Args, " "), err.Error())
	}

	var test_start_json SmartctlJsonOutputStartTest
	if err := json.Unmarshal(test_start_cmd_stdout, &test_start_json); err != nil {
		return nil, fmt.Errorf("could not parse command output for starting a scan: '%s' ", strings.Join(test_start_cmd.Args, " "))
	}
	if test_start_json.Smartctl.ExitStatus != 0 {
		return nil, fmt.Errorf("there seems to be an error when getting drive details: '%s' ", strings.Join(test_start_cmd.Args, " "))
	}

	return &test_start_json, nil
}

// smartctl --xall --json <device>
func SmartctlXall(device string) (*SmartctlJsonOutputXall, error) {
	// Find smartctl
	smartctl_executable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable in $PATH: %s , are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	// Build and execute command
	xall_cmd := exec.Command(smartctl_executable, "--xall", "--json", device)
	xall_cmd_stdout, err := xall_cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("running command for getting drive details: '%s' failed with exit code: '%d' and stderr: '%s'", strings.Join(xall_cmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}
		return nil, fmt.Errorf("running command for getting drive details: '%s' failed with error: '%s' ", strings.Join(xall_cmd.Args, " "), err.Error())
	}

	// Parse command output
	var xall_json SmartctlJsonOutputXall
	if err := json.Unmarshal(xall_cmd_stdout, &xall_json); err != nil {
		return nil, fmt.Errorf("could not parse command output for getting drive details: '%s' ", strings.Join(xall_cmd.Args, " "))
	}
	if xall_json.Smartctl.ExitStatus != 0 {
		return nil, fmt.Errorf("there seems to be an error when getting drive details: '%s' ", strings.Join(xall_cmd.Args, " "))
	}

	return &xall_json, nil
}

// smartctl --scan-open --json
func SmartctlScanOpen() (*SmartctlJsonOutputScanOpen, error) {
	smartctl_executable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable in $PATH: %s, are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	scan_open_cmd := exec.Command(smartctl_executable, "--scan-open", "--json")

	scan_open_cmd_stdout, err := scan_open_cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Output runs the command and returns its standard output. Any returned error will usually be of type [*ExitError]. If c.Stderr was nil and the returned error is of type [*ExitError], Output populates the Stderr field of the returned error.
			return nil, fmt.Errorf("running command: '%s' failed with exit code: '%d' and stderr: '%s'", strings.Join(scan_open_cmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}
		return nil, fmt.Errorf("running command: '%s' failed with an error: '%s'", strings.Join(scan_open_cmd.Args, " "), err.Error())
	}

	var scan_open_json SmartctlJsonOutputScanOpen
	if err := json.Unmarshal(scan_open_cmd_stdout, &scan_open_json); err != nil {
		return nil, fmt.Errorf("could not parse command output: '%s' ", strings.Join(scan_open_cmd.Args, " "))
	}

	return &scan_open_json, nil
}

// The types defined bellow are most likely not perfect.
// Some fields are always present, while the others are not.
// If a value is optional in a struct, it should be a pointer to a type. The json parser then can set it to nil if its not present
// If it is directly a type, it will be default initialized. This might be confusing, the user would not know if that value was really parsed from an existing field or simply default initialized
// I did not find a schema that describes the smartctl output. This classification of mandatory/optional fields are done by looking at example outputs.

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
	Name     string `json:"name"`
	InfoName string `json:"info_name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
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
	// The self test logs are either reported as "extended" or "standard"
	Extended *struct {
		Revision uint64 `json:"revision"`
		Count    uint64 `json:"count"`
		// Not always reported
		Sectors *uint64 `json:"sectors"`
		// Not always reported
		ErrorCountTotal *uint64 `json:"error_count_total"`
		// Not always reported
		ErrorCountOutdated *uint64 `json:"error_count_outdated"`
		// The table does not exist if the count is 0
		Table *[]struct {
			Type   SmartctlJsonOutputValueString `json:"type"`
			Status struct {
				Value  uint64 `json:"value"`
				String string `json:"string"`
				Passed bool   `json:"passed"`
			}
			LifetimeHours uint64 `json:"lifetime_hours"`
		} `json:"table"`
	}
	// The self test logs are either reported as "extended" or "standard"
	Standard *struct {
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
		Status struct {
			// Value is non-zero if a test is running?
			Value uint64 `json:"value"`
			// "completed without error"
			String string `json:"string"`
			// Only present if a self-test was completed
			Passed *bool `json:"passed"`
			// Only present if a self-test is running
			RemainingPercent *uint64 `json:"remaining_percent"`
		} `json:"status"`
		// Not always reported
		PollingMinutes *struct {
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
type SmartctlJsonOutputStartTest struct {
	JsonFormatVersion []uint64                    `json:"json_format_version"`
	Smartctl          SmartctlJsonOutputSmartctl  `json:"smartctl"`
	Device            SmartctlJsonOutputDevice    `json:"device"`
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
	ScsiVendor *string `json:"scsi_vendor"`
	// Not always reported
	ScsiProduct *string `json:"scsi_product"`
	// Not always reported
	ScsiRevision *string `json:"scsi_revision"`
	// Not always reported
	ScsiVersion *string `json:"scsi_version"`
	// Not always reported
	ModelFamily *string `json:"model_family"`
	// Not always reported
	ModelName *string `json:"model_name"`
	// Seems to be always present?
	SerialNumber string `json:"serial_number"`
	Wwn          *struct {
		Naa uint64 `json:"naa"`
		Oui uint64 `json:"oui"`
		Id  uint64 `json:"id"`
	} `json:"wwn"`
	FirmwareVersion *string `json:"firmware_version"`
	// Requires NVMe drive
	NvmePciVendor *struct {
		Id          uint64 `json:"id"`
		SubsystemId uint64 `json:"subsystem_id"`
	} `json:"nvme_pci_vendor"`
	// Requires NVMe drive
	NvmeIeeeOuiIdentifier *uint64 `json:"nvme_ieee_oui_identifier"`
	// Requires NVMe drive
	NvmeControllerId *uint64 `json:"nvme_controller_id"`
	// Requires NVMe drive
	NvmeVersion *SmartctlJsonOutputValueString `json:"nvme_version"`
	// Requires NVMe drive
	NvmeNumberOfNamespaces *uint64 `json:"nvme_number_of_namespaces"`
	// Requires NVMe drive
	NvmeNamespaces *[]SmartctlJsonOutputNvmeNamespaces `json:"nvme_namespaces"`
	// Requires NVMe drive
	NvmeSmartHealthInformationLog *SmarctlJsonOutputNvmeSmartHealthInformationLog `json:"nvme_smart_health_information_log"`
	// Requires NVMe drive
	NvmeErrorInformationLog *struct {
		Size   uint64 `json:"size"`
		Read   uint64 `json:"read"`
		Unread uint64 `json:"unread"`
	} `json:"nvme_error_information_log"`
	// Requires NVMe drive
	NvmeSelfTestLog *SmartctlJsonOutputNvmeSelfTestLog `json:"nvme_self_test_log"`
	// Seems to be always present?
	UserCapacity SmartctlJsonOutputBlocksBytes `json:"user_capacity"`
	// Seems to be always present?
	LogicalBlockSize uint64 `json:"logical_block_size"`
	// SSDs seem to be missing this attribute
	PhysicalBlockSize *uint64 `json:"physical_block_size"`
	// Requires spinning hard drive obviously
	RotationRate *uint64 `json:"rotation_rate"`
	// Not always reported
	FormFactor *struct {
		AtaValue uint64 `json:"ata_value"`
		Name     string `json:"name"`
	} `json:"form_factor"`
	// Requires flash storage
	Trim *struct {
		Supported bool `json:"supported"`
	} `json:"trim"`
	// Not always present, but if it is it seems to be true?
	InSmartctlDatabase *bool `json:"in_smartctl_database"`
	// Requires ATA connection
	AtaVersion *struct {
		String     string `json:"string"`
		MajorValue uint64 `json:"major_value"`
		MinorValue uint64 `json:"minor_value"`
	} `json:"ata_version"`
	// Requires ATA connection
	AtaSecurity struct {
		State            uint64 `json:"state"`
		String           string `json:"string"`
		Enabled          bool   `json:"enabled"`
		Frozen           bool   `json:"frozen"`
		MasterPasswordId uint64 `json:"master_password_id"`
	} `json:"ata_security"`
	// Requires ATA connection
	AtaSmartSelfTestLog *SmartctlJsonOutputAtaSmartSelfTestLog `json:"ata_smart_self_test_log"`
	// Requires ATA connection
	AtaSmartSelectiveSelfTestLog *SmartctlJsonOutputAtaSmartSelectiveSelfTestlog `json:"ata_smart_selective_self_test_log"`
	// Requires ATA connection
	AtaSmartData       *SmartctlJsonOutputAtaSmartData `json:"ata_smart_data"`
	AtaSctCapabilities *struct {
		Value                         uint64 `json:"value"`
		ErrorRecoveryControlSupported bool   `json:"error_recovery_control_supported"`
		FeatureControlSupported       bool   `json:"feature_control_supported"`
		DataTableSupported            bool   `json:"data_table_supported"`
	} `json:"ata_sct_capabilities"`
	// Requires ATA connection
	AtaSmartErrorLog *struct {
		Summary struct {
			Revision uint64 `json:"revision"`
			Count    uint64 `json:"count"`
		} `json:"summary"`
	} `json:"ata_smart_error_log"`
	AtaSmartAttributes *struct {
		Revision uint64                           `json:"revision"`
		Table    []SmartctlJsonOutputAtaAttribute `json:"table"`
	}
	// Requires SATA connection
	SataVersion *struct {
		String string `json:"string"`
		Value  uint64 `json:"value"`
	} `json:"sata_version"`
	// Seems to be reported alongside ata_version and sata_version only
	InterfaceSpeed *struct {
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
	// Not always reported
	ReadLookahead struct {
		Enabled bool `json:"enabled"`
	} `json:"read_lookahead"`
	// Not always reported
	WriteCache struct {
		Enabled bool `json:"write_cache"`
	} `json:"write_cache"`
	// Always reported
	SmartSupport struct {
		Available bool `json:"available"`
		// Only present if "available" is true
		Enabled *bool `json:"enabled"`
	} `json:"smart_support"`
	// Only present if the smart_support.enabled is true
	SmartStatuts *SmartctlJsonOutputSmartStatus `json:"smart_status"`
	// Top level "temperature" field seems to be present.
	Temperature struct {
		// Only the current value seems to be there. This temperature might be wrongfully reported as 0, as observed with some RAID controller cards.
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
	// Not always reported
	PowerCycleCount *uint64 `json:"power_cycle_count"`
	// Not always reported
	PowerOnTime struct {
		Hours   uint64 `json:"hours"`
		Minutes uint64 `json:"minutes"`
	} `json:"power_on_time"`

	// TODO: Implement these fields
	// ScsiErrorCounterLog *SmartctlJsonOutputScsiEr
	// ScsiGrownDefectList
	// ScsiStartStopCycleCounter
	// ScsiBackgroundScan
	// ScsiSasPort0 - N ????? Does it go up
}
