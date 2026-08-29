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
			ObjectDpdcy: false, // Default to false
		},
		Scripts: make(map[string]string),
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
				return nil, fmt.Errorf("invalid format on line: '%s', expected 'key = value'", line)
			}
			
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			val := strings.TrimSpace(parts[1])

			switch currentBlock {
			case "config.setup":
				if err := parseConfigSetup(config, key, val); err != nil {
					return nil, fmt.Errorf("error in [config.setup]: %w", err)
				}
			case "config.dependency":
				if err := parseConfigDependency(config, key, val); err != nil {
					return nil, fmt.Errorf("error in [config.dependency]: %w", err)
				}
			case "config.scripts":
				config.Scripts[key] = val
			default:
				return nil, fmt.Errorf("unknown block: '[%s]'", currentBlock)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filepath, err)
	}

	return config, nil
}

func parseConfigSetup(config *models.GomakeConfig, key, val string) error {
	switch key {
	case "compiler":
		config.Setup.Compiler = val
	case "flags":
		config.Setup.Flags = val
	case "name":
		config.Setup.Name = val
	default:
		return fmt.Errorf("unknown variable '%s'", key)
	}
	return nil
}

func parseConfigDependency(config *models.GomakeConfig, key, val string) error {
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
	case "object.dpdcy":
		valLower := strings.ToLower(val)
		if valLower == "yes" || valLower == "true" {
			config.Dependency.ObjectDpdcy = true
		} else if valLower == "no" || valLower == "false" || valLower == "" {
			config.Dependency.ObjectDpdcy = false
		} else {
			return fmt.Errorf("invalid value '%s' for object.dpdcy, expected 'yes' or 'no'", val)
		}
	default:
		return fmt.Errorf("unknown variable '%s'", key)
	}
	return nil
}
