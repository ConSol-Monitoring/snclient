package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stdout, "Invalid state argument. Please provide one of: 0, 1, 2, 3")
		os.Exit(3)
	}

	message := ""
	if len(os.Args) > 2 {
		message = os.Args[2]
	}
	if message != "" {
		message = ": " + message
	}

	exitStatus := 0
	switch os.Args[1] {
	case "0":
		fmt.Fprintf(os.Stdout, "OK%s", message)
	case "1":
		fmt.Fprintf(os.Stdout, "WARNING%s", message)
		exitStatus = 1
	case "2":
		fmt.Fprintf(os.Stdout, "CRITICAL%s", message)
		exitStatus = 2
	case "3":
		fmt.Fprintf(os.Stdout, "UNKNOWN%s", message)
		exitStatus = 3
	default:
		fmt.Fprintf(os.Stdout, "Invalid state argument. Please provide one of: 0, 1, 2, 3")
		exitStatus = 3
	}

	os.Exit(exitStatus)
}
