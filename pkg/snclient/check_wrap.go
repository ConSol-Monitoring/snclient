package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
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
	argList, err := ParseArgs(args, &l.data)
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

	re := regexp.MustCompile(`[%$](\w+)[%$]`)
	matches := re.FindAllStringSubmatch(formattedCommand, -1)

	for _, match := range matches {
		r := regexp.MustCompile(regexp.QuoteMeta(match[0]))
		formattedCommand = r.ReplaceAllString(formattedCommand, scriptArgs[match[1]])
	}

	var output []byte
	//nolint:gosec // tainted input is known and unavoidable
	switch runtime.GOOS {
	case "windows":
		output, err = exec.Command(winExecutable, "Set-ExecutionPolicy -Scope Process Unrestricted -Force;"+formattedCommand).Output()
	case "linux":
		output, err = exec.Command(formattedCommand).Output()
	}

	if err != nil {
		re := regexp.MustCompile(`exit status (\d)`)
		match := re.FindStringSubmatch(string(output))
		if len(match) > 0 {
			state, _ = strconv.ParseInt(match[1], 10, 64)
		} else {
			state = 3
			output = []byte(fmt.Sprintf("Unknown Error in Script: %s", err))
		}
	}

	return &CheckResult{
		State:  state,
		Output: string(output),
	}, nil
}
