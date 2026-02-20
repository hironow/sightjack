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
	rootCmd.SetArgs(cmd.DefaultToScan(rootCmd, os.Args[1:]))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
