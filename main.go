package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Anhdeface/gmake/internal/converter"
	"github.com/Anhdeface/gmake/internal/generator"
	"github.com/Anhdeface/gmake/internal/parser"
	"github.com/Anhdeface/gmake/internal/template"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	arg := os.Args[1]

	switch arg {
	case "convert":
		handleConvert(os.Args[2:])
	case "genconfig":
		generateTemplate()
	case "all":
		processAllConfigs()
	case "version":
		fmt.Println("gomake version 0.2.0-beta")
	default:
		processSingleConfig(arg)
	}
}

func printUsage() {
	fmt.Println("Gomake Transpiler & Converter")
	fmt.Println("Usage:")
	fmt.Println("  gomake convert [flags] [file] - Convert existing Makefile to .gomake format")
	fmt.Println("  gomake genconfig             - Generate a default build.gomake template")
	fmt.Println("  gomake all                   - Find and process all .gomake files concurrently")
	fmt.Println("  gomake version               - Print version information")
	fmt.Println("  gomake <filename.gomake>     - Process a specific .gomake file")
}

func handleConvert(args []string) {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	var input string
	var output string
	var toStdout bool
	var force bool
	var verbose bool

	fs.StringVar(&input, "i", "", "Input Makefile path (default: Makefile)")
	fs.StringVar(&input, "input", "", "Input Makefile path (default: Makefile)")
	fs.StringVar(&output, "o", "build.gomake", "Output .gomake file path")
	fs.StringVar(&output, "output", "build.gomake", "Output .gomake file path")
	fs.BoolVar(&toStdout, "s", false, "Output to stdout")
	fs.BoolVar(&toStdout, "stdout", false, "Output to stdout")
	fs.BoolVar(&force, "f", false, "Force overwrite existing output file")
	fs.BoolVar(&force, "force", false, "Force overwrite existing output file")
	fs.BoolVar(&verbose, "v", false, "Verbose mode")
	fs.BoolVar(&verbose, "verbose", false, "Verbose mode")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(2)
	}

	if input == "" {
		if fs.NArg() > 0 {
			input = fs.Arg(0)
		} else {
			input = "Makefile"
		}
	}

	if toStdout {
		res, err := converter.ConvertFileToStdout(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Conversion failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(res)
		return
	}

	if err := converter.ConvertFile(input, output, force); err != nil {
		fmt.Fprintf(os.Stderr, "Conversion failed: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Successfully converted '%s' to '%s'\n", input, output)
	} else {
		fmt.Printf("Successfully generated %s\n", output)
	}
}

func generateTemplate() {
	err := os.WriteFile("build.gomake", []byte(template.ConfigTemplate), 0644)
	if err != nil {
		fmt.Printf("Error writing template: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully generated build.gomake template")
}

func processAllConfigs() {
	files, err := filepath.Glob("*.gomake")
	if err != nil {
		fmt.Printf("Error searching for .gomake files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No .gomake files found in the current directory.")
		return
	}

	var wg sync.WaitGroup
	var hasError bool
	var mu sync.Mutex

	for _, file := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			err := transpile(filename)
			if err != nil {
				fmt.Printf("Error processing %s: %v\n", filename, err)
				mu.Lock()
				hasError = true
				mu.Unlock()
			}
		}(file)
	}

	wg.Wait()
	if hasError {
		fmt.Println("Finished processing with errors.")
		os.Exit(1)
	}
	fmt.Println("Finished processing all configurations.")
}

func processSingleConfig(filename string) {
	if err := transpile(filename); err != nil {
		fmt.Printf("Error processing %s: %v\n", filename, err)
		os.Exit(1)
	}
}

func transpile(filename string) error {
	config, err := parser.ParseConfig(filename)
	if err != nil {
		return err
	}

	outputFile := filename + ".makefile"
	err = generator.GenerateMakefile(config, outputFile)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully generated %s from %s\n", outputFile, filename)
	return nil
}
