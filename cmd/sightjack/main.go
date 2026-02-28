package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	sightjack "github.com/hironow/sightjack"
	cmd "github.com/hironow/sightjack/internal/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	// NOTE: RewriteBoolFlags was removed intentionally (MY-336, per MY-334 consensus).
	// The rewriter converted "--dry-run false" → "--dry-run=false", overriding pflag's
	// NoOptDefVal behavior. This was identified as non-POSIX in MY-334 cross-tool review
	// and all 4 tools (phonewave, amadeus, paintress, sightjack) agreed to remove it.
	// Standard behavior: "--dry-run false" = --dry-run (true) + positional "false".
	// To explicitly set false, use the equals form: "--dry-run=false".
	args := cmd.DefaultToScan(rootCmd, os.Args[1:])
	rootCmd.SetArgs(args)

	ctx, cancel := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(sightjack.ExitCode(err))
	}
}
