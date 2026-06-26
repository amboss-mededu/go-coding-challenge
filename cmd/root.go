package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amboss-mededu/go-coding-challenge/internal/generator"
	"github.com/spf13/cobra"
)

var (
	schemasDir string
	outputDir  string
)

var rootCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Go types from JSON Schema files",
	Long: `Reads JSON Schema files from the given directory and produces
one Go source file per schema in the output directory.`,
	RunE: runGenerate,
}

func init() {
	rootCmd.Flags().StringVarP(&schemasDir, "schemas", "s", "./schemas", "directory containing JSON Schema files")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "./model", "output directory for generated Go files")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runGenerate(cmd *cobra.Command, args []string) error {
	g, err := generator.NewGenerator(schemasDir)
	if err != nil {
		return err
	}
	files, err := g.Generate()
	if err != nil {
		return err
	}
	for filename, src := range files {
		path := filepath.Join(outputDir, filename)
		if err := os.WriteFile(path, src, 0644); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
	}
	return nil
}
