package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		//nolint:forbidigo // use of `fmt.Println` forbidden by pattern `^(fmt\.Print(|f|ln)|print|println)$`
		fmt.Println("Invalid state argument. Please provide one of: 0, 1, 2, 3")
		os.Exit(3)
	}

	state, err := strconv.Atoi(os.Args[1])
	if err != nil || state < 0 || state > 3 {
		//nolint:forbidigo // dito
		fmt.Println("Invalid state argument. Please provide one of: 0, 1, 2, 3")
		os.Exit(3)
	}

	var message string
	if len(os.Args) > 2 {
		message = ": " + os.Args[2]
	}

	exitStatus := 0
	switch state {
	case 0:
		//nolint:forbidigo // dito
		fmt.Println("OK" + message)
	case 1:
		//nolint:forbidigo // dito
		fmt.Println("WARNING" + message)
		exitStatus = 1
	case 2:
		//nolint:forbidigo // dito
		fmt.Println("CRITICAL" + message)
		exitStatus = 2
	case 3:
		//nolint:forbidigo // dito
		fmt.Println("UNKNOWN" + message)
		exitStatus = 3
	}

	os.Exit(exitStatus)
}
