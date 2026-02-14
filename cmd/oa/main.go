package main

import (
	"os"

	"github.com/bnema/openai-accounts-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
