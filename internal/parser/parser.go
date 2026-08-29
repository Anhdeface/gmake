package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Anhdeface/gmake/internal/models"
)

// ParseConfig reads the given .gomake file and returns a GomakeConfig object.
func ParseConfig(filepath string) (*models.GomakeConfig, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	defer file.Close()

	config := &models.GomakeConfig{
		Dependency: models.ConfigDependency{
			OFix: true, // Default to true
		},
	}

	scanner := bufio.NewScanner(file)
	currentBlock := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Handle comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for EOF marker
		if line == "./gomake" {
			break
		}

		// Check for block start/end
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			blockName := strings.ToLower(line[1 : len(line)-1])
			if blockName == "end" {
				currentBlock = ""
			} else {
				currentBlock = blockName
			}
			continue
		}

		// Parse key-value pairs
		if currentBlock != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue // skip invalid lines
			}
			
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			val := strings.TrimSpace(parts[1])

			switch currentBlock {
			case "config.name":
				parseConfigName(config, key, val)
			case "config.dependency":
				parseConfigDependency(config, key, val)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filepath, err)
	}

	return config, nil
}

func parseConfigName(config *models.GomakeConfig, key, val string) {
	switch key {
	case "compiler":
		config.Name.Compiler = val
	case "flags":
		config.Name.Flags = val
	case "name":
		config.Name.Name = val
	}
}

func parseConfigDependency(config *models.GomakeConfig, key, val string) {
	switch key {
	case "target":
		config.Dependency.Target = val
	case "sources":
		// split by space for multiple sources
		sources := strings.Fields(val)
		config.Dependency.Sources = append(config.Dependency.Sources, sources...)
	case "includes":
		includes := strings.Fields(val)
		// Process includes: strip trailing /* if any
		for _, inc := range includes {
			if strings.HasSuffix(inc, "/*") {
				inc = inc[:len(inc)-2]
			}
			config.Dependency.Includes = append(config.Dependency.Includes, inc)
		}
	case "o.fix":
		if strings.ToLower(val) == "no" || strings.ToLower(val) == "false" {
			config.Dependency.OFix = false
		} else {
			config.Dependency.OFix = true
		}
	}
}
