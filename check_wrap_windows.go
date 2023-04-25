package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

func init() {
	AvailableChecks["check_wrap"] = CheckEntry{"check_wrap", new(CheckWrap)}
}

type CheckWrap struct {
	noCopy noCopy
	data   CheckData
}

/* check_service todo
 * todo .
 */
func (l *CheckWrap) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args, &l.data)
	var script string

	// parse treshold args
	for _, arg := range argList {
		if arg.key == "script" {
			script = arg.value
		}
	}

	// collect script output
	output, err := exec.Command("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", script).Output()
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
