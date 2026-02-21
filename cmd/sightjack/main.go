package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cmd "github.com/hironow/sightjack/internal/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	// NOTE: No RewriteBoolFlags step — removed intentionally (MY-336).
	// pflag bool flags use NoOptDefVal; "--dry-run false" is parsed as
	// --dry-run (true) + positional "false", per POSIX/GNU convention.
	// Use "--dry-run=false" (equals form) to explicitly set false.
	args := cmd.DefaultToScan(rootCmd, os.Args[1:])
	rootCmd.SetArgs(args)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
