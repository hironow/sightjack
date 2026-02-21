// Command docgen generates Markdown CLI documentation from cobra commands.
//
// Usage:
//
//	go run ./internal/tools/docgen
package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/hironow/sightjack/internal/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	rootCmd.DisableAutoGenTag = true

	outDir := "./docs/cli"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("failed to create output dir: %v", err)
	}

	if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
		log.Fatalf("failed to generate docs: %v", err)
	}

	log.Printf("CLI docs generated in %s", outDir)
}
