// Package main is the entry point for xrayctl.
package main

import (
	"os"

	"xrayctl/cli"
)

func main() {
	switch cli.ParseFlags() {
	case -1:
		cli.ShowMenu()
	case 1:
		os.Exit(1)
	}
}
