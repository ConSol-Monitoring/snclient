package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type CheckWrap struct {
	noCopy        noCopy
	data          CheckData
	commandString string
}

/* check_wrap_windows todo
 * todo .
 */
func (l *CheckWrap) Check(_ *Agent, args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	var err error
	argList, err := l.data.ParseArgs(args)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	scriptArgs := map[string]string{}
	formattedCommand := l.commandString
	winExecutable := "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"

	for _, arg := range argList {
		switch arg.key {
		case "SCRIPT", "script":
			scriptArgs["SCRIPT"] = arg.value
		case "ARGS", "args":
			scriptArgs["ARGS"] = arg.value
		}
	}

	if _, exists := scriptArgs["ARGS"]; !exists {
		args := []string{}
		for _, arg := range argList {
			args = append(args, arg.key)
		}
		scriptArgs["ARGS"] = strings.Join(args, " ")
	}
	re := regexp.MustCompile(`[%$](\w+)[%$]`)
	matches := re.FindAllStringSubmatch(formattedCommand, -1)

	for _, match := range matches {
		r := regexp.MustCompile(regexp.QuoteMeta(match[0]))
		formattedCommand = r.ReplaceAllString(formattedCommand, scriptArgs[match[1]])
	}

	var scriptOutput []byte
	//nolint:gosec // tainted input is known and unavoidable
	switch runtime.GOOS {
	case "windows":
		scriptOutput, err = exec.Command(winExecutable, "Set-ExecutionPolicy -Scope Process Unrestricted -Force;"+formattedCommand+"; $LASTEXITCODE").CombinedOutput()
	case "linux":
		scriptOutput, err = exec.Command(formattedCommand+"; echo $?").CombinedOutput()
	}

	var output string
	re = regexp.MustCompile(`(\d+)\s*\z`)
	match := re.FindStringSubmatch(string(scriptOutput))
	if len(match) > 0 {
		state, _ = strconv.ParseInt(match[1], 10, 64)
		output = re.ReplaceAllString(string(scriptOutput), "")
	} else {
		state = 3
		output = fmt.Sprintf("Unknown Error in Script: %s", err)
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
