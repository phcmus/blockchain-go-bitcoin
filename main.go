package main

import (
	"os"

	"github.com/phnaharris/harris-blockchain-token/cli"
)

func main() {
	defer os.Exit(0)

	cmd := cli.CommandLine{}
	cmd.Run()
}
