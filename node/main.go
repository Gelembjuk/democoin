package main

import (
	"fmt"
	"os"

	"github.com/gelembjuk/democoin/lib"
)

func main() {
	// Parse input
	input, ierr := GetAppInput()

	if ierr != nil {
		// something went wrong when parsing input data
		fmt.Printf("Error: %s\n", ierr.Error())
		os.Exit(0)
	}

	fmt.Printf("%s - %s\n\n", lib.ApplicationTitle, lib.ApplicationVersion)

	if input.checkNeedsHelp() {
		// if user requested a help, display it
		input.printUsage()
		os.Exit(0)
	}
	// create node client object
	// this will create all other objects needed to execute a command
	cli := getNodeCLI(input)

	if cli.isInteractiveMode() {
		// it is command to display results right now
		err := cli.ExecuteCommand()

		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}
		os.Exit(0)
	}

	if cli.isNodeManageMode() {
		// it is the command to manage node server
		err := cli.ExecuteManageCommand()

		if err != nil {
			fmt.Printf("Node Manage Error: %s\n", err.Error())
		}

		os.Exit(0)
	}

	fmt.Println("Unknown command!")
	input.printUsage()
}
