package main

import (
	"stacking/cmd"
)

// Version is set via ldflags during build
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
