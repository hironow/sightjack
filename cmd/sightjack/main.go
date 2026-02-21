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
	args := cmd.DefaultToScan(rootCmd, os.Args[1:])
	args = cmd.RewriteBoolFlags(rootCmd, args)
	rootCmd.SetArgs(args)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
