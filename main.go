package main

import (
	"blockchain/cli"
	"os"
)

func main() {
	defer os.Exit(0)

	cli := cli.CommandLine{}
	cli.Run()

}
