package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func mustBool(cmd *cobra.Command, name string) bool {
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not defined: %v", name, err))
	}
	return v
}

func mustString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not defined: %v", name, err))
	}
	return v
}

func mustDuration(cmd *cobra.Command, name string) time.Duration {
	v, err := cmd.Flags().GetDuration(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not defined: %v", name, err))
	}
	return v
}
