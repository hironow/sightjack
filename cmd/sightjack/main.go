package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	cmd "github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()

	rootCmd := cmd.NewRootCommand()
	args := os.Args[1:]
	if cmd.NeedsDefaultScan(rootCmd, args) {
		args = append([]string{"scan"}, args...)
	} else {
		args = cmd.ReorderArgs(rootCmd, args)
	}
	rootCmd.SetArgs(args)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		var silent *domain.SilentError
		if !errors.As(err, &silent) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return domain.ExitCode(err)
}
