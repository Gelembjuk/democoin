package main

import (
	"fmt"
	"os"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/wallet"
)

func main() {
	input, ierr := GetAppInput()

	if ierr != nil {
		fmt.Printf("Error: %s\n", ierr.Error())
		os.Exit(0)
	}

	if checkNeedsHelp(input) {
		printUsage()
		os.Exit(0)
	}
	if checkConfigUpdateNeeded(input) {
		updateConfig(input.DataDir, input)
		os.Exit(0)
	}

	logger := lib.CreateLogger()

	if input.LogDest != "stdout" {
		logger.LogToFiles(input.DataDir, "log_trace.txt", "log_info.txt", "log_warning.txt", "log_error.txt")
	}

	walletscli := wallet.WalletCLI{}
	walletscli.Init(logger, input)

	walletscli.NodeMode = false

	err := walletscli.ExecuteCommand()

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
