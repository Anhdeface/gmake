package converter

import (
	"fmt"
	"os"

	"github.com/Anhdeface/gmake/internal/models"
)

// ConvertContent translates Makefile raw text content directly into .gomake configuration string.
func ConvertContent(content string) (string, error) {
	ast, err := ParseMakefile(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse Makefile: %w", err)
	}

	cfg, err := ConvertAST(ast)
	if err != nil {
		return "", fmt.Errorf("failed to convert Makefile AST: %w", err)
	}

	return FormatConfig(cfg)
}

// ConvertMakefile is an alias for ConvertContent.
func ConvertMakefile(content string) (string, error) {
	return ConvertContent(content)
}

// ConvertFile reads a Makefile from inputPath, converts it, and writes the .gomake to outputPath.
// If overwrite is false and outputPath already exists, returns an error.
func ConvertFile(inputPath, outputPath string, overwrite bool) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input Makefile '%s': %w", inputPath, err)
	}

	if !overwrite {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("output file '%s' already exists (use -f to overwrite)", outputPath)
		}
	}

	outContent, err := ConvertContent(string(data))
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(outContent), 0644); err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", outputPath, err)
	}

	return nil
}

// ConvertFileToStdout reads a Makefile from inputPath, converts it, and returns the .gomake string.
func ConvertFileToStdout(inputPath string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read input Makefile '%s': %w", inputPath, err)
	}

	return ConvertContent(string(data))
}

// ConvertASTToModel translates a parsed MakefileAST into GomakeConfig models.
func ConvertASTToModel(ast *MakefileAST) (*models.GomakeConfig, error) {
	return ConvertAST(ast)
}
