package snclient

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Generates a config file, where snclient can call a script.
// scriptName does not have an extension
// scriptFilename does have (most likely an OS specific) script extension.
// It registers four commands for script
// scriptName_arg1 : ./${SCRIPT_FILENAME} "$ARG1$"
// scriptName_arg_numbered : ./${SCRIPT_FILENAME} "$ARG1$" "$ARG2$" "$ARG3$" "$ARG4$" "$ARG5$" "$ARG6$" "$ARG7$" "$ARG8$" "$ARG9$" "$ARG10$"
// scriptName_args : ./${SCRIPT_FILENAME} "$ARGS$"
// scriptName_args_quouted : ./${SCRIPT_FILENAME} "$ARGS"$"
//
//nolint:unparam // scriptName is so far always "powershell_detail" , no other test script uses this function. Keep it as a parameter for future use.
func snclientConfigFileWithScript(t *testing.T, scriptsDir, scriptName, scriptFilename string) string {
	t.Helper()

	configTemplate := `
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = ${SCRIPTS_DIR}
shared-path = %(scripts)

[/settings/external scripts]
timeout = 1111111
allow arguments = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_arg1 = ./${SCRIPT_FILENAME} $ARG1$

[/settings/external scripts/scripts/${SCRIPT_NAME}_arg1]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_arg_numbered = ./${SCRIPT_FILENAME} $ARG1$ $ARG2$ $ARG3$ $ARG4$ $ARG5$ $ARG6$ $ARG7$ $ARG8$ $ARG9$ $ARG10$

[/settings/external scripts/scripts/${SCRIPT_NAME}_arg_numbered]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_args = ./${SCRIPT_FILENAME} $ARGS$

[/settings/external scripts/scripts/${SCRIPT_NAME}_args]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_args_quouted = ./${SCRIPT_FILENAME} $ARGS"$

[/settings/external scripts/scripts/${SCRIPT_NAME}_args_quouted]
allow arguments = true
allow nasty characters = true
`

	mapper := func(placeholderName string) string {
		switch placeholderName {
		case "SCRIPTS_DIR":
			return scriptsDir
		case "SCRIPT_NAME":
			return scriptName
		case "SCRIPT_FILENAME":
			return scriptFilename
		default:
			// if its not some value we know, leave it as is
			return "$" + placeholderName
		}
	}

	return os.Expand(configTemplate, mapper)
}

func TestMakeCmd(t *testing.T) {
	config := ""
	snc := StartTestAgent(t, config)

	commandString := `.\t\scripts\powershell_detail.ps1 -option1 option1 -option2 'option2' -option3 "option3" -option4 'option4.option4,option4:option4;option4|option4$option4' `
	cmd, err := snc.makeCmd(context.TODO(), commandString)

	require.NoErrorf(t, err, "there should not be any errors when converting command: %s into an exec.Cmd of os/exec", commandString)

	assert.NotEmptyf(t, cmd.SysProcAttr.CmdLine, "exec.Cmd from command: %s should not have an empty SysProcAttr.CmdLine", commandString)

	// cmd.Args is unused if cmd.SysProcAttr.CmdLine is set and nonempty
	// snclient sets it and does not populate cmd.Args
	// expectedArgs := []string{
	// 	`./scripts/test/test_script.ps1`,
	// 	`-option1`,
	// 	`option1`,
	// 	`-option2`,
	// 	`'option2'`,
	// 	`-option3`,
	// 	`"option3"`,
	// 	`-option4`,
	// 	`'option4.option4,option4:option4;option4|option4$option4'`,
	// }
	// assert.Equalf(t, expectedArgs, cmd.Args, "converted exec.Cmd from command: %s should have these args: %v", commandString, expectedArgs)

	// the quoutes should not be removed
	// the reasoning is to pass some arguments as written inside the quoutes, so that they can take a string form and not be converted
	// if an argument is passed like this --optionX foo,bar powershell parameter parser thiks it is a string array and refuses to parse it as string
	// users have to use it like this --optionX 'foo,bar' to have it accepted as a string

	cmdLineExpectedContains := `-option1 option1 -option2 'option2' -option3 "option3" -option4 'option4.option4,option4:option4;option4|option4$option4'`
	assert.Containsf(t, cmd.SysProcAttr.CmdLine,
		cmdLineExpectedContains,
		"exec.Cmd from command: %s\nshould contain this substring: %s\nbut it looks like this: %s",
		commandString, cmdLineExpectedContains, cmd.SysProcAttr.CmdLine)

	var pathEnv string
	for _, envVar := range cmd.Env {
		if strings.HasPrefix(envVar, "PATH=") {
			pathEnv = envVar
		}
	}

	assert.NotEmpty(t, pathEnv, "converted exec.Cmd from command: %s should contain PATH environment variable", commandString)

	// script is found under C:\Users\sorus\repositories\snclient\pkg\snclient\t
	// scriptsPath, _ := snc.config.Section("/paths").GetString("scripts")
	// assert.Containsf(t, pathEnv, scriptsPath+":", "converted exec.Cmd from command: %s should have its PATH variable: %s include the config ScriptsPath: %s", commandString, pathEnv, scriptsPath)

	assert.Truef(t, cmd.SysProcAttr.HideWindow, "converted exec.Cmd from command: %s should hide its spawned window", commandString)

	StopTestAgent(t, snc)
}

func TestPowershell1(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "powershell_detail"
	scriptFilename := "powershell_detail.ps1"

	config := snclientConfigFileWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// simulate a default call of the script with no arguments
	res := snc.RunCheck("powershell_detail_arg1", []string{})

	outputString := string(res.BuildPluginOutput())

	assert.Equalf(t, CheckExitOK, res.State, "check should return state ok")

	// the string rawCommandLine: <value> is printed from the powershell_detail script. Invoke it locally and get snippets from its output
	rawCommandlineExpected := []string{
		`Raw Commandline: `,
		`t\scripts\powershell_detail.ps1`,
	}

	for _, rawCommandLineExpectedItem := range rawCommandlineExpected {
		assert.Containsf(t, outputString, rawCommandLineExpectedItem, "raw commandline should contain: %s", rawCommandLineExpectedItem)
	}
}

func TestPowershellScriptArg1(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "powershell_detail"
	scriptFilename := scriptName + ".ps1"
	scriptMacroType := "_arg1"
	scriptArgs := []string{"-option1 option1 -option2 'option2' -option3  \"option3\" -option4 'foo,bar' -option5 \"baz,xyz\" "}

	config := snclientConfigFileWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// simulate a call from check_nsc_web. this calls the (snc *Agent).runCheck directly, skipping over RunCheck
	// argument macros are evaluated after this function,
	// call different registered versions of the script command
	// this one is using the default one
	checkResult, checkData := snc.runCheck(context.TODO(), scriptName+scriptMacroType, scriptArgs, 0, nil, false, false)
	assert.NotNilf(t, checkResult, "check should return a checkResult")
	assert.NotNilf(t, checkData, "check should return a checkData")

	outputString := string(checkResult.BuildPluginOutput())

	assert.Equalf(t, CheckExitOK, checkResult.State, "check should return state OK")

	// raw commandline seems to strip options with double quotation marks
	// the solution: if you want to pass something literally, use single quotation marks

	rawCommandlineExpected := []string{
		`Raw Commandline: `,
		`t\scripts\powershell_detail.ps1`,
		`-option1 option1`,
		`-option2 'option2'`,
		`-option3 option3`,
		`-option4 'foo,bar`,
		`-option5 baz,xyz`,
		`Bound Parameter | Name: option1 | Type: String | Value: option1`,
		`Bound Parameter | Name: option2 | Type: String | Value: option2`,
		`Bound Parameter | Name: option3 | Type: String | Value: option3`,
		`Bound Parameter | Name: option4 | Type: String | Value: foo,bar`,
		`Bound Parameter | Name: option5 | Type: Object[] | Value: baz xyz`,
	}

	for _, rawCommandLineExpectedItem := range rawCommandlineExpected {
		assert.Containsf(t, outputString, rawCommandLineExpectedItem, "raw commandline should contain: %s", rawCommandLineExpectedItem)
	}
}

//nolint:dupl // the functions are largely the same, but scriptMacroType is different. Redefining expected strings for each macro type is easier to understand.
func TestPowershellScriptArgNumbered(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "powershell_detail"
	scriptFilename := scriptName + ".ps1"
	scriptMacroType := "_arg_numbered"
	scriptArgs := []string{
		"-option1",
		"option1",
		"-option2",
		"option2",
		"-option3",
		"\"option3\"",
		"-option4",
		"'foo,bar'",
		"-option5",
		"\"baz,xyz\"",
	}

	config := snclientConfigFileWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// call different registered versions of the script command
	// this one is using the arg_numbered
	checkResult, checkData := snc.runCheck(context.TODO(), scriptName+scriptMacroType, scriptArgs, 0, nil, false, false)
	assert.NotNilf(t, checkResult, "check should return a checkResult")
	assert.NotNilf(t, checkData, "check should return a checkData")

	outputString := string(checkResult.BuildPluginOutput())

	assert.Equalf(t, CheckExitOK, checkResult.State, "check should return state OK")

	rawCommandlineExpected := []string{
		`Raw Commandline: `,
		`t\scripts\powershell_detail.ps1`,
		`-option1 option1`,
		`-option2 option2`,
		`-option3 option3`,
		`-option4 'foo,bar'`,
		`-option5 baz,xyz`,
		`Bound Parameter | Name: option1 | Type: String | Value: option1`,
		`Bound Parameter | Name: option2 | Type: String | Value: option2`,
		`Bound Parameter | Name: option3 | Type: String | Value: option3`,
		`Bound Parameter | Name: option4 | Type: String | Value: foo,bar`,
		`Bound Parameter | Name: option5 | Type: Object[] | Value: baz xyz`,
	}

	for _, rawCommandLineExpectedItem := range rawCommandlineExpected {
		assert.Containsf(t, outputString, rawCommandLineExpectedItem, "raw commandline should contain: %s", rawCommandLineExpectedItem)
	}
}

//nolint:dupl // the functions are largely the same, but scriptMacroType is different. Redefining expected strings for each macro type is easier to understand.
func TestPowershellScriptArgs(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "powershell_detail"
	scriptFilename := scriptName + ".ps1"
	scriptMacroType := "_args"
	scriptArgs := []string{
		"-option1",
		"option1",
		"-option2",
		"'option2'",
		"-option3",
		"\"option3\"",
		"-option4",
		"'foo,bar'",
		"-option5",
		"\"baz,xyz\"",
	}

	config := snclientConfigFileWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// call different registered versions of the script command
	// this one is using the arg_numbered
	checkResult, checkData := snc.runCheck(context.TODO(), scriptName+scriptMacroType, scriptArgs, 0, nil, false, false)
	assert.NotNilf(t, checkResult, "check should return a checkResult")
	assert.NotNilf(t, checkData, "check should return a checkData")

	outputString := string(checkResult.BuildPluginOutput())

	assert.Equalf(t, CheckExitOK, checkResult.State, "check should return state OK")

	rawCommandlineExpected := []string{
		`Raw Commandline: `,
		`t\scripts\powershell_detail.ps1`,
		`-option1 option1`,
		`-option2 'option2'`,
		`-option3 option3`,
		`-option4 'foo,bar'`,
		`-option5 baz,xyz`,
		`Bound Parameter | Name: option1 | Type: String | Value: option1`,
		`Bound Parameter | Name: option2 | Type: String | Value: option2`,
		`Bound Parameter | Name: option3 | Type: String | Value: option3`,
		`Bound Parameter | Name: option4 | Type: String | Value: foo,bar`,
		`Bound Parameter | Name: option5 | Type: Object[] | Value: baz xyz`,
	}

	for _, rawCommandLineExpectedItem := range rawCommandlineExpected {
		assert.Containsf(t, outputString, rawCommandLineExpectedItem, "raw commandline should contain: %s", rawCommandLineExpectedItem)
	}
}

func TestPowershellScriptArgsQuouted(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "powershell_detail"
	scriptFilename := scriptName + ".ps1"
	scriptMacroType := "_args_quouted"
	scriptArgs := []string{
		"-option1",
		"option1",
		"-option2",
		"'option2'",
		"-option3",
		"\"option3\"",
		"-option4",
		"'foo,bar'",
		"-option5",
		"\"baz,xyz\"",
	}

	config := snclientConfigFileWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// call different registered versions of the script command
	// this one is using the arg_numbered
	checkResult, checkData := snc.runCheck(context.TODO(), scriptName+scriptMacroType, scriptArgs, 0, nil, false, false)
	assert.NotNilf(t, checkResult, "check should return a checkResult")
	assert.NotNilf(t, checkData, "check should return a checkData")

	outputString := string(checkResult.BuildPluginOutput())
	t.Logf("\n%s\n", outputString)

	assert.Equalf(t, CheckExitOK, checkResult.State, "check should return state OK")

	// args_quouted uses macro $ARGS"$

	rawCommandlineExpected := []string{
		`Raw Commandline: `,
		`t\scripts\powershell_detail.ps1`,
		`-option1 option1`,
		`-option2 'option2'`,
		`-option3 "option3"`,
		`-option4 'foo,bar'`,
		`-option5 "baz,xyz"`,
		`Bound Parameter | Name: option1 | Type: String | Value: option1`,
		`Bound Parameter | Name: option2 | Type: String | Value: option2`,
		`Bound Parameter | Name: option3 | Type: String | Value: option3`,
		`Bound Parameter | Name: option4 | Type: String | Value: foo,bar`,
		`Bound Parameter | Name: option5 | Type: String | Value: baz,xyz`,
	}

	for _, rawCommandLineExpectedItem := range rawCommandlineExpected {
		assert.Containsf(t, outputString, rawCommandLineExpectedItem, "raw commandline should contain: %s", rawCommandLineExpectedItem)
	}
}
