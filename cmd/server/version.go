package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	BuildID = "nil"
)

func versionCmd() *cobra.Command {
	versionCmd := cobra.Command{
		Use:   "version",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("build: %s\n", BuildID)
		},
	}
	return &versionCmd
}
