package snclient

import (
	"context"
	"errors"
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
	driveFilter CommaStringList
	// Test to run. Should be one of these values: 'offline' , 'short' , 'long' , 'conveyance' , 'select'
	testType string
	// Logical block address to start the test on. Has to be a number. Required if the test type is 'select'
	testStart uint64
	// End block of the test. Can also be specified as 'max' or a number. Required if the test type is 'select'
	testEndLba string
}

var validTestTypes = []string{"offline", "short", "long", "conveyance", "select"}

func NewCheckDriveHealth() CheckHandler {
	return &CheckDriveHealth{
		driveFilter: make([]string, 0),
		testType:    "offline",
		testStart:   0,
		testEndLba:  "0",
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
				value: &checkDriveHealth.driveFilter,
				description: "Drives to check health for. Give it as a comma separated list of logical device names e.g '/dev/sda,'/dev/nvme0' ." +
					" Leaving it empty will check all drives which report a SMART status.",
			},
			"test_type": {
				value:       &checkDriveHealth.testType,
				description: fmt.Sprintf("SMART test type to perform for checking the health of the drives. Available test types are: '%s' ", strings.Join(validTestTypes, ",")),
			},
			"test_start_lba": {
				value:       &checkDriveHealth.testStart,
				description: "Logical block address to start the test on. Has to be specified as a number. Needed if the test type is 'select'",
			},
			"test_end_lba": {
				value:       &checkDriveHealth.testEndLba,
				description: "Logical block address to end the test on, inclusive. Can be specified as a number or as 'max' to select the last block on the disk. Required if the test type is 'select'",
			},
		},
		defaultCritical: " return_code != '0' && test_result != 'PASSED' && smart_health_status != 'PASSED' ",
		defaultWarning:  " return_code != '0' || test_result != 'PASSED' || smart_health_status != 'PASSED' ",
		detailSyntax: "drive: %(test_drive) | test: %(test_type) $(test_start_lba)-%(test_end_lba) | test_result: %(test_result) | " +
			"smart_health_status: %(smart_health_status) | return_code: %(return_code) (%(return_code_explanation))",
		okSyntax:    "%(status) - All %(count) drives are ok",
		topSyntax:   "%(status) - %(problem_count)/%(count) drives , %(problem_list)",
		emptySyntax: "Failed to find any drives that the filter and smartctl could work with",
		emptyState:  CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "test_type", description: "Type of SMART test that was performed."},
			{name: "test_drive", description: "The drive the test was performed on"},
			{name: "test_result", description: "The result of the test. Takes possible outputs: \"PASSED\" , \"FAILED\" , \"UNKNOWN\" ."},
			{name: "test_details", description: "The details of the test given by smartctl."},
			{name: "test_start_lba", description: "Logical block address the test was started on. Required if test type is 'select' "},
			{name: "test_end_lba", description: "Logical block address the test was ended on. Required if the test type is 'select' "},
			{name: "smart_health_status", description: "SMART overall health self-assesment done by the firmware with the current SMART attributes. " +
				" It is evaluated independently from the test result, but is just as important. Takes possible values: \"OK\" , \"FAIL\" , \"UNKNOWN\" ."},
			{name: "return_code", description: "The return code status of the smartctl command used to get drive details after the test was done."},
			{name: "return_code_explanation", description: "Explanation of the return code of the smartctl command used to get drive details after the test was done."},
		},
		exampleDefault: `
Perform an offline test on all drives
		check_drive_health

Perform a short test on a specific NVMe drive
		check_drive_health test_type='short' drive_filter='/dev/nvme0'
		`,
	}
}

func (checkDriveHealth *CheckDriveHealth) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	scanOpenOutput, err := SmartctlScanOpen()
	if err != nil {
		return nil, fmt.Errorf("error when discovering smartctl devices : %s", err.Error())
	}

	var driveFilter []string
	if driveFilterArgument, ok := check.args["drive_filter"]; ok {
		if driveFilterCslPtr, ok := driveFilterArgument.value.(*CommaStringList); ok {
			cslValue := *driveFilterCslPtr
			driveFilter = []string(cslValue)
		} else {
			return nil, fmt.Errorf("unexpected type for drive_filter_argument.value , expected *CommaStringList")
		}
	}

	entryBase := map[string]string{}
	entryBase["test_type"] = checkDriveHealth.testType
	entryBase["test_start_lba"] = strconv.FormatUint(checkDriveHealth.testStart, 10)
	entryBase["test_end_lba"] = checkDriveHealth.testEndLba

	if !slices.Contains(validTestTypes, entryBase["test_type"]) {
		return nil, fmt.Errorf("unexpected test type to perform: %s , valid test types are: %s", entryBase["test_type"], strings.Join(validTestTypes, ","))
	}

	var testString string
	if entryBase["test_type"] == "select" {
		if entryBase["test_start_lba"] == "" || entryBase["test_end_lba"] == "" {
			return nil, fmt.Errorf("if the test type is 'select', the test_start_lba and test_end_lba arguments have to be specified")
		}

		testString = entryBase["test_type"] + "," + entryBase["test_start_lba"] + "-" + entryBase["test_end_lba"]
	} else {
		testString = entryBase["test_type"]
	}

	var waitGroup sync.WaitGroup
	type SmartctlTestResult struct {
		device            SmartctlJSONOutputDevice
		xallJSON          *SmartctlJSONOutputXall
		err               error
		xallCmdReturnCode int64
	}
	testResultsChannel := make(chan SmartctlTestResult, len(scanOpenOutput.Devices))

	for _, device := range scanOpenOutput.Devices {
		if len(driveFilter) > 0 && !slices.Contains(driveFilter, device.Name) {
			// skip this device
			log.Debugf("Skipping the device :%s as it is not in the filter: %s", device.Name, strings.Join(driveFilter, ","))

			continue
		}

		waitGroup.Add(1)

		// Define and call an asynchronus function, returns a channel result
		go func(device SmartctlJSONOutputDevice) {
			// This will decrement the waitgroup counter before returning
			defer waitGroup.Done()

			result := SmartctlTestResult{device: device}
			xallJSON, xallCmdReturnCode, xallJSONError := SmartctlXall(device.Name)

			switch {
			case xallJSONError != nil:
				result.err = fmt.Errorf("error when getting details about the drive: %s , error : %s", device.Name, xallJSONError.Error())
			case !xallJSON.SmartSupport.Available:
				result.err = fmt.Errorf("device does not support SMART: %s", device.Name)
			case xallJSON.SmartSupport.Enabled != nil && !*xallJSON.SmartSupport.Enabled:
				result.err = fmt.Errorf("device does supports SMART, but does not have it enabled: %s", device.Name)
			default:
				xallJSON, xallJSONError = SmartctlTestAndAwaitCompletion(device.Name, testString)
				result.xallJSON = xallJSON
				result.err = xallJSONError
				result.xallCmdReturnCode = xallCmdReturnCode
			}

			testResultsChannel <- result
		}(device)
	}

	go func() {
		// will wait until counter reaches 0
		waitGroup.Wait()
		// close the channel afterwards, no more results can be sent there
		close(testResultsChannel)
	}()

	// collect them in a map, since some devices might be skipped due to filtering
	collectedResultsMap := make(map[string]SmartctlTestResult)
	for result := range testResultsChannel {
		collectedResultsMap[result.device.Name] = result
	}

	// Add results as data points to the check
	for _, result := range collectedResultsMap {
		entryResult := maps.Clone(entryBase)
		entryResult["test_drive"] = result.device.Name
		entryResult["return_code"] = strconv.FormatInt(result.xallCmdReturnCode, 10)
		entryResult["return_code_explanation"], _ = SmartctlParseReturnCode(result.xallCmdReturnCode)

		if result.err != nil {
			return nil, fmt.Errorf("encountered an error while performing the test and awaiting results for device: %s , error: %w", entryResult["test_drive"], result.err)
		}

		// There are three results we can use to determine the health

		// 1. Test result -> offline, short, long etc.
		// This depends on the disk type, so I am writing a helper function to get it

		entryResult["test_result"], entryResult["test_details"] = GetLatestTestResult(result.xallJSON)

		// 2. SmartStatus.Passed if it exists.
		// That seems to be the SMART overall-health self-assessment test result

		if result.xallJSON.SmartStatuts == nil {
			entryResult["smart_health_status"] = "UNKNOWN"
		}
		if result.xallJSON.SmartStatuts.Passed {
			entryResult["smart_health_status"] = "OK"
		} else {
			entryResult["smart_health_status"] = "FAIL"
		}

		// 3. Return code of the smartctl information check command.
		// Smartctl returns 0 if everything looks ok, anything else points to a warning or critical issue
		// This was added as an attribute already

		check.listData = append(check.listData, entryResult)
	}

	for _, data := range check.listData {
		AddCheckMetrics(check, data)
	}

	return check.Finalize()
}

func AddCheckMetrics(check *CheckData, data map[string]string) {
	var testResultPassed int32
	if data["test_result"] == "PASSED" {
		testResultPassed = 1
	}
	if check.HasThreshold("test_result") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "test_result_passed",
				Name:          fmt.Sprintf("drive: %s test_result_passed: %d", data["test_drive"], testResultPassed),
				Value:         testResultPassed,
				Unit:          "",
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
		)
	}
	var smartHealthStatusPassed int32
	if data["smart_health_status"] == "PASSED" {
		smartHealthStatusPassed = 1
	}
	if check.HasThreshold("smart_health_status") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "smart_health_status_passed",
				Name:          fmt.Sprintf("drive: %s smart_health_status_passed: %d", data["test_drive"], smartHealthStatusPassed),
				Value:         smartHealthStatusPassed,
				Unit:          "",
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
		)
	}
	returnCode, returnCodeParseError := strconv.ParseInt(data["return_code"], 10, 64)
	if returnCodeParseError != nil {
		log.Warnf("could not parse return code value into int64 for drive: %s , value: %s", data["test_drive"], data["return_code"])
	}
	var returnCodeOk int32
	if returnCode == 0 {
		returnCodeOk = 1
	}
	if check.HasThreshold("return_code") && returnCodeParseError == nil {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "return_code_ok",
				Name:          fmt.Sprintf("drive: %s return_code_ok: %d", data["test_drive"], returnCodeOk),
				Value:         returnCodeOk,
				Unit:          "",
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
		)
	}
}

func GetLatestTestResult(xallJSON *SmartctlJSONOutputXall) (result, details string) {
	switch xallJSON.Device.Type {
	case "scsi":
		return "UNKNOWN", fmt.Sprintf("getting the latest test result for drive type %s is not yet implemented", xallJSON.Device.Type)
	case "sat":
		// Need to access xall_json.AtaSmartData
		if xallJSON.AtaSmartData == nil {
			return "UNKNOWN", fmt.Sprintf("smartctl drive details does not have ata_smart_data, cant get latest test details: %s", xallJSON.Device.Name)
		}
		if xallJSON.AtaSmartData.SelfTest.Status.Passed == nil {
			return "UNKNOWN", fmt.Sprintf("smartctl drive details does not have ata_smart_data.self_test.status.passed , cant get result the latest completed test: %s", xallJSON.Device.Name)
		}
		if *xallJSON.AtaSmartData.SelfTest.Status.Passed {
			return "PASSED", xallJSON.AtaSmartData.SelfTest.Status.String
		}

		return "FAILED", xallJSON.AtaSmartData.SelfTest.Status.String
	case "nvme":
		if xallJSON.NvmeSelfTestLog == nil {
			return "UNKNOWN", fmt.Sprintf("smarctl drive details does not have nvme_self_test_log, cant get latest test details: %s", xallJSON.Device.Name)
		}
		if len(xallJSON.NvmeSelfTestLog.Table) == 0 {
			return "UNKNOWN", fmt.Sprintf("smarctl drive details nvme_self_test_log.table is empty, cant get latest test details: %s", xallJSON.Device.Name)
		}
		if xallJSON.NvmeSelfTestLog.Table[0].SelfTestResult.Value == 0 && xallJSON.NvmeSelfTestLog.Table[0].SelfTestResult.String == "Completed without error" {
			return "PASSED", xallJSON.NvmeSelfTestLog.Table[0].SelfTestResult.String
		}

		return "FAILED", xallJSON.NvmeSelfTestLog.Table[0].SelfTestResult.String
	default:
		return "UNKNOWN", fmt.Sprintf("getting the latest test result for drive type %s is not yet implemented", xallJSON.Device.Type)
	}
}

// Starts the specified test, then awaits until it is done, and then returns the latest xall output it gets
func SmartctlTestAndAwaitCompletion(device, testString string) (*SmartctlJSONOutputXall, error) {
	// Start a test

	var startTestJSON *SmartctlJSONOutputStartTest
	var startTestError error
	if startTestJSON, startTestError = SmartctlStartTest(device, testString); startTestError != nil {
		return nil, fmt.Errorf("error when starting a test: %s", startTestError.Error())
	}

	deviceType := startTestJSON.Device.Type
	log.Debugf("Started test on device: %s with the device type: %s", startTestJSON.Device.Name, startTestJSON.Device.Type)

	var xallJSON *SmartctlJSONOutputXall
	var xallJSONError error

	// Wait 100 ms after starting the test. Offline tests will finish very quickly, we can check right away
	time.Sleep(time.Millisecond * 100)

	// Busy loop until the test is complete?
busyloop_test_completion:
	for {
		// Get the latest values from device
		if xallJSON, _, xallJSONError = SmartctlXall(device); xallJSONError != nil {
			return nil, fmt.Errorf("error when getting device details: %s", xallJSONError.Error())
		}

		switch deviceType {
		case "scsi":
			return nil, fmt.Errorf("testing not yet implemented for type: %s", deviceType)
		case "sat":

			if xallJSON.AtaSmartData == nil {
				return nil, fmt.Errorf("ata_smart_data is not present xall output")
			}
			if xallJSON.AtaSmartData.SelfTest.Status.Value != 0 {
				// the test is presumably still running
				break
			}
			if xallJSON.AtaSmartData.SelfTest.Status.RemainingPercent != nil {
				// the test is presumably still running
				break
			}

			break busyloop_test_completion
		case "nvme":
			if xallJSON.NvmeSelfTestLog == nil {
				return nil, fmt.Errorf("nvme_self_test_log is not present xall output")
			}
			if xallJSON.NvmeSelfTestLog.CurrentSelfTestOperation.Value == 1 {
				// the test is still running
				break
			}

			break busyloop_test_completion
		default:
			return nil, fmt.Errorf("testing not yet implemented for type: %s", deviceType)
		}

		// Wait after each check
		time.Sleep(time.Second * 10)
	}

	return xallJSON, nil
}

// smartctl --test <test_string> --json <device>
func SmartctlStartTest(device, testString string) (*SmartctlJSONOutputStartTest, error) {
	// Find smartctl
	smartctlExecutable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable in $PATH: %s , are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	// Start the test . A test can always be started without knowing the device details
	testStartCmd := exec.Command(smartctlExecutable, "--json", "--test", testString, device)
	testStartCmdStdout, err := testStartCmd.CombinedOutput()
	var testStartCmdReturnCode int64
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			testStartCmdReturnCode = int64(exitError.ExitCode())

			// It can happen that the smartctl command gives a return code other than 0, which is expected if there is a problem with the drive
			// We still have to differentiate if the return code meant no information could be gathered
			testStartCmdReturnCodeExplanation, testStartCmdReturnCodeIsError := SmartctlParseReturnCode(int64(exitError.ExitCode()))

			if testStartCmdReturnCodeIsError != nil {
				return nil, fmt.Errorf("running command for starting the test: '%s'."+
					" Got this return code: '%d'. "+
					" This is a code that prevents further information gathering: '%s'", strings.Join(testStartCmd.Args, " "), testStartCmdReturnCode, testStartCmdReturnCodeExplanation)
			}
		}
	}

	var testStartJSON SmartctlJSONOutputStartTest
	if err := json.Unmarshal(testStartCmdStdout, &testStartJSON); err != nil {
		return nil, fmt.Errorf("could not parse command output for starting a test: '%s' ", strings.Join(testStartCmd.Args, " "))
	}

	return &testStartJSON, nil
}

// smartctl --xall --json <device>
func SmartctlXall(device string) (xall *SmartctlJSONOutputXall, returnCode int64, err error) {
	// Find smartctl
	smartctlExecutable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, -1, fmt.Errorf("could not find smartctl executable in $PATH: %s , are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	// Build and execute command
	xallCmd := exec.Command(smartctlExecutable, "--xall", "--json", device)
	xallCmdStdout, err := xallCmd.CombinedOutput()
	var xallCmdReturnCode int64
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			xallCmdReturnCode = int64(exitError.ExitCode())

			// It can happen that the smartctl command gives a return code other than 0, which is expected if there is a problem with the drive
			// We still have to differentiate if the return code meant no information could be gathered
			xallCmdReturnCodeExplanation, xallCmdReturnCodeIsError := SmartctlParseReturnCode(int64(exitError.ExitCode()))

			if xallCmdReturnCodeIsError != nil {
				return nil, -1, fmt.Errorf("running command for information gathering: '%s'."+
					" Got this return code: '%d'. "+
					" This is a code that prevents further information gathering: '%s'", strings.Join(xallCmd.Args, " "), xallCmdReturnCode, xallCmdReturnCodeExplanation)
			}
		}
	}

	// Parse command output
	var xallJSON SmartctlJSONOutputXall
	if err := json.Unmarshal(xallCmdStdout, &xallJSON); err != nil {
		return nil, -1, fmt.Errorf("could not parse command output for getting drive details: '%s' ", strings.Join(xallCmd.Args, " "))
	}

	return &xallJSON, xallCmdReturnCode, nil
}

// smartctl --scan-open --json
func SmartctlScanOpen() (*SmartctlJSONOutputScanOpen, error) {
	smartctlExecutable, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("could not find smartctl executable in $PATH: %s, are you running as a priviledged user: %s", os.Getenv("PATH"), err.Error())
	}

	scanOpenCmd := exec.Command(smartctlExecutable, "--scan-open", "--json")

	scanOpenCmdStdout, err := scanOpenCmd.CombinedOutput()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("running command for scanning drives: '%s'."+
				" Failed with exit code: '%d'. "+
				" Stderr: '%s'", strings.Join(scanOpenCmd.Args, " "), exitError.ExitCode(), string(exitError.Stderr))
		}

		return nil, fmt.Errorf("running command: '%s' failed with an error: '%s'", strings.Join(scanOpenCmd.Args, " "), err.Error())
	}

	var scanOpenJSON SmartctlJSONOutputScanOpen
	if err := json.Unmarshal(scanOpenCmdStdout, &scanOpenJSON); err != nil {
		return nil, fmt.Errorf("could not parse command output: '%s' ", strings.Join(scanOpenCmd.Args, " "))
	}

	return &scanOpenJSON, nil
}

// Parse and explain the return code of the smartctl.
// The first error is set if smartctl could not get information about the device.
// Return code can be non-zero even if smartctl could read something.
// The string describe the return code
func SmartctlParseReturnCode(returnCode int64) (explanations string, err error) {
	if returnCode == 0 {
		return "No problem with the error code", nil
	}

	// Bits are indexed from 0 to 7, even bit 0 being 1 is an error.
	// https://linux.die.net/man/8/smartctl

	var bitmaskBit0 int64 = 0b00000001
	commandLineDidNotParse := (returnCode & bitmaskBit0) > 0
	if commandLineDidNotParse {
		return "Command line did not parse", fmt.Errorf("command line did not parse")
	}

	var bitmaskBit1 int64 = 0b00000010
	deviceOpenFailed := (returnCode & bitmaskBit1) > 0
	if deviceOpenFailed {
		return "device open failed, device did not return an IDENTIFY DEVICE structure, or device is in a low-power mode",
			fmt.Errorf("device open failed, device did not return an IDENTIFY DEVICE structure, or device is in a low-power mode")
	}

	var errorExplanations []string

	var bitmaskBit2 int64 = 0b00000100
	smartOrAtaCommandFailed := (returnCode & bitmaskBit2) > 0
	if smartOrAtaCommandFailed {
		errorExplanations = append(errorExplanations, "Some SMART or other ATA command to the disk failed, or there was a checksum error in the SMART data structure.")
	}

	var bitmaskBit3 int64 = 0b00001000
	smartStatusDiskFailing := (returnCode & bitmaskBit3) > 0
	if smartStatusDiskFailing {
		errorExplanations = append(errorExplanations, "SMART status check returned DISK FAILING.")
	}

	var bitmaskBit4 int64 = 0b00010000
	someAttributesBellowPrefailThreshold := (returnCode & bitmaskBit4) > 0
	if someAttributesBellowPrefailThreshold {
		errorExplanations = append(errorExplanations, "Some Attributes have been <= threshold , which translates into a prefailure.")
	}
	var bitmaskBit5 int64 = 0b00100000
	someAttributesBellowThresholdInPast := (returnCode & bitmaskBit5) > 0
	if someAttributesBellowThresholdInPast {
		errorExplanations = append(errorExplanations, "SMART status check returned DISK OK but some (usage or prefail) Attributes have been <= threshold at some time in the past.")
	}

	var bitmaskBit6 int64 = 0b01000000
	deviceErrorLogContainsErrors := (returnCode & bitmaskBit6) > 0
	if deviceErrorLogContainsErrors {
		errorExplanations = append(errorExplanations, "The device error log contains records of errors.")
	}

	var bitmaskBit7 int64 = 0b10000000
	selfTestLogContainsErrors := (returnCode & bitmaskBit7) > 0
	if selfTestLogContainsErrors {
		errorExplanations = append(errorExplanations, "The device self-test log contains records of errors, [ATA only] Failed self-tests outdated by a newer successful extended self-test are ignored.")
	}

	return "The return code of the smartctl indicates problems: " + strings.Join(errorExplanations, " | "), nil
}

// The types defined bellow are most likely not perfect.
// Some fields are always present, while the others are not.
// If a value is optional in a struct, it should be a pointer to a type. The json parser then can set it to nil if its not present
// If it is directly a type, it will be default initialized. This might be confusing, the user would not know if that value was really parsed from an existing field or simply default initialized
// I did not find a schema that describes the smartctl output. This classification of mandatory/optional fields are done by looking at example outputs.

type SmartctlJSONOutputSmartctl struct {
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

type SmartctlJSONOutputDevice struct {
	Name     string `json:"name"`
	InfoName string `json:"info_name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}

type SmartctlJSONOutputLocalTime struct {
	TimeT   uint64 `json:"time_t"`
	Asctime string `json:"asctime"`
}

type SmartctlJSONOutputNvmeNamespaces struct {
	ID               uint64                        `json:"id"`
	Size             SmartctlJSONOutputBlocksBytes `json:"size"`
	Capacity         SmartctlJSONOutputBlocksBytes `json:"capacity"`
	Utilization      SmartctlJSONOutputBlocksBytes `json:"utilization"`
	FormattedLbaSize uint64                        `json:"formatted_lba_base"`
	Eui64            struct {
		Oui   uint64 `json:"oui"`
		ExtID uint64 `json:"ext_id"`
	} `json:"eui64"`
}

type SmartctlJSONOutputBlocksBytes struct {
	Blocks uint64 `json:"id"`
	Bytes  uint64 `json:"bytes"`
}

type SmartctlJSONOutputValueString struct {
	Value  uint64 `json:"value"`
	String string `json:"string"`
}

type SmartctlJSONOutputSmartStatus struct {
	Passed bool `json:"passed"`
	Nvme   *struct {
		Value uint64 `json:"value"`
	} `json:"nvme"`
}

type SmarctlJSONOutputNvmeSmartHealthInformationLog struct {
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

type SmartctlJSONOutputNvmeSelfTestLogTableEntry struct {
	SelfTestCode   SmartctlJSONOutputValueString `json:"self_test_code"`
	SelfTestResult SmartctlJSONOutputValueString `json:"self_test_result"`
	PowerOnHours   uint64                        `json:"power_on_hours"`
}

type SmartctlJSONOutputNvmeSelfTestLog struct {
	CurrentSelfTestOperation SmartctlJSONOutputValueString                 `json:"current_self_test_operation"`
	Table                    []SmartctlJSONOutputNvmeSelfTestLogTableEntry `json:"table"`
}

type SmartctlJSONOutputAtaSmartSelfTestLog struct {
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
			Type   SmartctlJSONOutputValueString `json:"type"`
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

type SmartctlJSONOutputAtaSmartSelectiveSelfTestlog struct {
	Revision uint64 `json:"revision"`
	Table    []struct {
		LbaMin uint64 `json:"lba_min"`
		LbaMax uint64 `json:"lba_max"`
		Status SmartctlJSONOutputValueString
	} `json:"table"`
	Flags struct {
		Value               uint64 `json:"value"`
		ReminderScanEnabled bool   `json:"reminder_scan_enabled"`
	}
	PowerUPScanResumeMinutes uint64 `json:"power_up_scan_resume_minutes"`
}

type SmartctlJSONOutputAtaSmartData struct {
	OfflineDataCollection struct {
		Status            SmartctlJSONOutputValueString `json:"status"`
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

type SmartctlJSONOutputAtaAttribute struct {
	ID         uint64 `json:"id"`
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
	Raw SmartctlJSONOutputValueString `json:"raw"`
}

// smartctl --scan-open json
type SmartctlJSONOutputScanOpen struct {
	JSONFormatVersion []uint64                   `json:"json_format_version"`
	Smartctl          SmartctlJSONOutputSmartctl `json:"smartctl"`
	Devices           []SmartctlJSONOutputDevice `json:"devices"`
}

// smartctl --json --test short /dev/nvme0
type SmartctlJSONOutputStartTest struct {
	JSONFormatVersion []uint64                    `json:"json_format_version"`
	Smartctl          SmartctlJSONOutputSmartctl  `json:"smartctl"`
	Device            SmartctlJSONOutputDevice    `json:"device"`
	LocalTime         SmartctlJSONOutputLocalTime `json:"local_time"`
}

// smartctl --json --xall /dev/sda
type SmartctlJSONOutputXall struct {
	// Always present
	JSONFormatVersion []uint64 `json:"json_format_version"`
	// Always present
	Smartctl SmartctlJSONOutputSmartctl `json:"smartctl"`
	// Always present
	LocalTime SmartctlJSONOutputLocalTime `json:"local_time"`
	// Might be missing if you type the device name wrong
	Device SmartctlJSONOutputDevice `json:"device"`
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
		ID  uint64 `json:"id"`
	} `json:"wwn"`
	FirmwareVersion *string `json:"firmware_version"`
	// Requires NVMe drive
	NvmePciVendor *struct {
		ID          uint64 `json:"id"`
		SubsystemID uint64 `json:"subsystem_id"`
	} `json:"nvme_pci_vendor"`
	// Requires NVMe drive
	NvmeIeeeOuiIdentifier *uint64 `json:"nvme_ieee_oui_identifier"`
	// Requires NVMe drive
	NvmeControllerID *uint64 `json:"nvme_controller_id"`
	// Requires NVMe drive
	NvmeVersion *SmartctlJSONOutputValueString `json:"nvme_version"`
	// Requires NVMe drive
	NvmeNumberOfNamespaces *uint64 `json:"nvme_number_of_namespaces"`
	// Requires NVMe drive
	NvmeNamespaces *[]SmartctlJSONOutputNvmeNamespaces `json:"nvme_namespaces"`
	// Requires NVMe drive
	NvmeSmartHealthInformationLog *SmarctlJSONOutputNvmeSmartHealthInformationLog `json:"nvme_smart_health_information_log"`
	// Requires NVMe drive
	NvmeErrorInformationLog *struct {
		Size   uint64 `json:"size"`
		Read   uint64 `json:"read"`
		Unread uint64 `json:"unread"`
	} `json:"nvme_error_information_log"`
	// Requires NVMe drive
	NvmeSelfTestLog *SmartctlJSONOutputNvmeSelfTestLog `json:"nvme_self_test_log"`
	// Seems to be always present?
	UserCapacity SmartctlJSONOutputBlocksBytes `json:"user_capacity"`
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
		MasterPasswordID uint64 `json:"master_password_id"`
	} `json:"ata_security"`
	// Requires ATA connection
	AtaSmartSelfTestLog *SmartctlJSONOutputAtaSmartSelfTestLog `json:"ata_smart_self_test_log"`
	// Requires ATA connection
	AtaSmartSelectiveSelfTestLog *SmartctlJSONOutputAtaSmartSelectiveSelfTestlog `json:"ata_smart_selective_self_test_log"`
	// Requires ATA connection
	AtaSmartData       *SmartctlJSONOutputAtaSmartData `json:"ata_smart_data"`
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
		Table    []SmartctlJSONOutputAtaAttribute `json:"table"`
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
	SmartStatuts *SmartctlJSONOutputSmartStatus `json:"smart_status"`
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

	// I have seen these fields as well, but did not implement them yet
	// ScsiErrorCounterLog *SmartctlJsonOutputScsiEr
	// ScsiGrownDefectList
	// ScsiStartStopCycleCounter
	// ScsiBackgroundScan
	// ScsiSasPort0 - N ????? Does it go up
}
