// Package cmd rival_export_reasons.go: cobra wiring for
// `sightjack rival export reasons` (Phase 1.1B).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
//
// The subcommand:
//
//   - reads a Rival Contract v1 specification D-Mail (--input <path>)
//     OR resolves the current revision for a wave id from the local
//     archive (--wave <id>);
//   - applies usecase.ExportToReasonsCanvas (markdown) or
//     usecase.ExportToReasonsCanvasJSON depending on --format;
//   - writes the result to --output (or stdout when omitted);
//   - rejects ContractConflict by default; --allow-conflict downgrades the
//     conflict to a stderr warning and uses the lexicographically smaller
//     D-Mail name for best-effort export (per plan §"`--wave` selection
//     rule").
//
// The handler is composition-root I/O glue: it wires session readers and
// the pure usecase projection. No D-Mail mutation, no LLM, no network.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase"
)

// rivalExportFormatMarkdown / rivalExportFormatJSON name the supported
// values of `--format`.
const (
	rivalExportFormatMarkdown = "markdown"
	rivalExportFormatJSON     = "json"
)

// newRivalExportReasonsCommand wires the `rival export reasons` cobra
// command. All flags are local to this command; the cross-cutting flags
// (--config, --output text/json) are NOT used here — `--output` on this
// subcommand is a file path, not a global format selector.
func newRivalExportReasonsCommand() *cobra.Command {
	var (
		input         string
		wave          string
		outputPath    string
		format        string
		allowConflict bool
		baseDir       string
	)

	cmd := &cobra.Command{
		Use:   "reasons",
		Short: "Export a Rival Contract v1 spec as OpenSPDD REASONS Canvas",
		Long: `Project a Rival Contract v1 specification into OpenSPDD REASONS Canvas.

Reads either a stand-alone D-Mail file (--input) or the current revision
for a wave id resolved from the local archive (--wave). The two modes are
mutually exclusive.

Output is markdown by default; --format json emits a deterministic JSON
shape. The mapping is documented at:
  refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Mapping"`,
		Example: `  # Export a stand-alone D-Mail file to stdout
  sightjack rival export reasons --input ./spec-auth_aaaaaaaa.md

  # Export the current revision for a wave to a file
  sightjack rival export reasons --wave wave-auth-expiry --output canvas.md

  # JSON output for downstream tools
  sightjack rival export reasons --input ./spec.md --format json

  # Allow best-effort export under same-revision conflict
  sightjack rival export reasons --wave wave-x --allow-conflict`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != rivalExportFormatMarkdown && format != rivalExportFormatJSON {
				return fmt.Errorf("--format must be %q or %q, got %q",
					rivalExportFormatMarkdown, rivalExportFormatJSON, format)
			}
			if strings.TrimSpace(input) == "" && strings.TrimSpace(wave) == "" {
				return errors.New("one of --input or --wave is required")
			}

			source, err := loadRivalSource(cmd, input, wave, baseDir, allowConflict)
			if err != nil {
				return err
			}

			rendered, err := renderRivalCanvas(format, source)
			if err != nil {
				return err
			}

			if strings.TrimSpace(outputPath) != "" {
				if writeErr := os.WriteFile(outputPath, []byte(rendered), 0o600); writeErr != nil {
					return fmt.Errorf("write output %s: %w", outputPath, writeErr)
				}
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			return nil
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Path to a Rival Contract v1 specification D-Mail (.md)")
	cmd.Flags().StringVar(&wave, "wave", "", "Wave id to resolve from the local archive (mutually exclusive with --input)")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&format, "format", rivalExportFormatMarkdown, "Output format: markdown|json")
	cmd.Flags().BoolVar(&allowConflict, "allow-conflict", false, "Allow best-effort export under same-revision conflict (warns on stderr)")
	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Project base directory for --wave archive resolution (default: cwd)")

	cmd.MarkFlagsMutuallyExclusive("input", "wave")

	return cmd
}

// rivalExportSource carries everything ExportToReasonsCanvas needs.
type rivalExportSource struct {
	contract        harness.RivalContract
	metadata        harness.RivalContractMetadata
	sourceDMailName string
}

// loadRivalSource resolves either --input or --wave to a parsed
// (contract, metadata, source-name) tuple. Errors are returned for I/O
// failures, parse failures, contract conflicts (when allowConflict is
// false), and missing wave matches. Conflict warnings are emitted to
// cmd.ErrOrStderr() when allowConflict is true.
func loadRivalSource(cmd *cobra.Command, input, wave, baseDir string, allowConflict bool) (rivalExportSource, error) {
	if strings.TrimSpace(input) != "" {
		return loadRivalFromInputFile(input)
	}
	return loadRivalFromWaveArchive(cmd, wave, baseDir, allowConflict)
}

// loadRivalFromInputFile parses a single D-Mail file into the projection
// source tuple. The file must already be a Rival Contract v1
// specification (validated by the projection itself).
func loadRivalFromInputFile(path string) (rivalExportSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rivalExportSource{}, fmt.Errorf("read input %s: %w", path, err)
	}
	mail, err := session.ParseDMail(data)
	if err != nil || mail == nil {
		return rivalExportSource{}, fmt.Errorf("parse input %s: %w", path, err)
	}
	contract, ok, err := harness.ParseRivalContractBody(mail.Body)
	if err != nil {
		return rivalExportSource{}, fmt.Errorf("parse rival body %s: %w", path, err)
	}
	if !ok {
		return rivalExportSource{}, fmt.Errorf("input %s is not a Rival Contract v1 specification (missing # Contract: title)", path)
	}
	meta, ok, err := harness.ParseRivalContractMetadata(mail.Metadata)
	if err != nil {
		return rivalExportSource{}, fmt.Errorf("parse rival metadata %s: %w", path, err)
	}
	if !ok {
		return rivalExportSource{}, fmt.Errorf("input %s is missing rival-contract-v1 metadata (contract_schema)", path)
	}
	return rivalExportSource{
		contract:        contract,
		metadata:        meta,
		sourceDMailName: mail.Name,
	}, nil
}

// loadRivalFromWaveArchive runs the deterministic ProjectCurrentContracts
// projection over <baseDir>/.siren/archive/ and returns the current
// revision for the supplied wave id. ContractConflict at the wave is the
// only condition where allowConflict relaxes the failure into a
// best-effort warning.
func loadRivalFromWaveArchive(cmd *cobra.Command, wave, baseDir string, allowConflict bool) (rivalExportSource, error) {
	resolvedBase, err := resolveBaseDir(baseDir)
	if err != nil {
		return rivalExportSource{}, err
	}
	dmails, err := session.ReadArchiveDMails(resolvedBase)
	if err != nil {
		return rivalExportSource{}, err
	}
	current, conflicts := harness.ProjectCurrentContracts(dmails)

	// Conflict that targets THIS wave id is the only one that matters.
	for _, c := range conflicts {
		if c.ContractID != wave {
			continue
		}
		if !allowConflict {
			return rivalExportSource{}, fmt.Errorf("contract conflict for wave %q (%s): %s", wave, c.Reason, strings.Join(c.Names, ", "))
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: contract conflict for wave %q (%s); falling back to lexicographically smallest D-Mail name\n", wave, c.Reason)
		fallback, fallbackErr := pickConflictFallback(dmails, wave, c.Names)
		if fallbackErr != nil {
			return rivalExportSource{}, fallbackErr
		}
		return fallback, nil
	}

	for _, cc := range current {
		if cc.Metadata.ID == wave {
			return rivalExportSource{
				contract:        cc.Contract,
				metadata:        cc.Metadata,
				sourceDMailName: cc.DMailName,
			}, nil
		}
	}
	return rivalExportSource{}, fmt.Errorf("no Rival Contract v1 specification found for wave %q in %s", wave, resolvedBase)
}

// pickConflictFallback returns the rivalExportSource for the lexico-
// graphically smaller D-Mail name listed in a ContractConflict. Used only
// in the --allow-conflict best-effort path.
func pickConflictFallback(dmails []domain.DMail, wave string, conflictNames []string) (rivalExportSource, error) {
	if len(conflictNames) == 0 {
		return rivalExportSource{}, fmt.Errorf("contract conflict for wave %q: no candidate names", wave)
	}
	names := append([]string(nil), conflictNames...)
	sort.Strings(names)
	pick := names[0]

	for _, d := range dmails {
		if d.Name != pick {
			continue
		}
		contract, ok, err := harness.ParseRivalContractBody(d.Body)
		if err != nil || !ok {
			continue
		}
		meta, ok, err := harness.ParseRivalContractMetadata(d.Metadata)
		if err != nil || !ok {
			continue
		}
		return rivalExportSource{
			contract:        contract,
			metadata:        meta,
			sourceDMailName: d.Name,
		}, nil
	}
	return rivalExportSource{}, fmt.Errorf("contract conflict fallback failed for wave %q: candidate %q not parseable", wave, pick)
}

// renderRivalCanvas dispatches on format. Both branches are pure
// usecase calls.
func renderRivalCanvas(format string, src rivalExportSource) (string, error) {
	switch format {
	case rivalExportFormatJSON:
		return usecase.ExportToReasonsCanvasJSON(src.contract, src.metadata, src.sourceDMailName)
	default:
		return usecase.ExportToReasonsCanvas(src.contract, src.metadata, src.sourceDMailName)
	}
}

// resolveBaseDir returns an absolute base directory for archive lookup.
// When the flag is empty, falls back to os.Getwd().
func resolveBaseDir(baseDir string) (string, error) {
	if strings.TrimSpace(baseDir) != "" {
		return baseDir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}
	return wd, nil
}
