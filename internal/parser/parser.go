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
		Setups:       make(map[string]*models.ConfigSetup),
		Dependencies: make(map[string]*models.ConfigDependency),
		Scripts:      make(map[string]string),
	}

	scanner := bufio.NewScanner(file)
	currentBlock := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Handle comments (do not strip comments inside config.scripts block to preserve commands like curl //)
		if currentBlock != "config.scripts" {
			if idx := strings.Index(line, "//"); idx != -1 {
				line = line[:idx]
			}
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

		if currentBlock == "const" {
			parts := strings.Split(line, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					config.Constants = append(config.Constants, p)
				}
			}
			continue
		}

		// Parse key-value pairs
		if currentBlock != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid format on line: '%s', expected 'key = value'", line)
			}
			key := strings.TrimSpace(parts[0])
			if currentBlock != "config.scripts" {
				key = strings.ToLower(key)
			}
			val := strings.TrimSpace(parts[1])

			if currentBlock == "config.setup" || currentBlock == "config.dependency" {
				return nil, fmt.Errorf("missing suffix, e.g., [%s.app]", currentBlock)
			} else if strings.HasPrefix(currentBlock, "config.setup.") {
				suffix := strings.TrimPrefix(currentBlock, "config.setup.")
				if !contains(config.Constants, suffix) {
					return nil, fmt.Errorf("suffix '%s' is not declared in [const]", suffix)
				}
				if config.Setups[suffix] == nil {
					config.Setups[suffix] = &models.ConfigSetup{}
				}
				if err := parseConfigSetup(config.Setups[suffix], key, val); err != nil {
					return nil, fmt.Errorf("error in [%s]: %w", currentBlock, err)
				}
			} else if strings.HasPrefix(currentBlock, "config.dependency.") {
				suffix := strings.TrimPrefix(currentBlock, "config.dependency.")
				if !contains(config.Constants, suffix) {
					return nil, fmt.Errorf("suffix '%s' is not declared in [const]", suffix)
				}
				if config.Dependencies[suffix] == nil {
					config.Dependencies[suffix] = &models.ConfigDependency{
						ObjectDpdcy: false,
						BuildType:   "executable",
					}
				}
				if err := parseConfigDependency(config.Dependencies[suffix], key, val); err != nil {
					return nil, fmt.Errorf("error in [%s]: %w", currentBlock, err)
				}
			} else if currentBlock == "config.scripts" {
				if key == "" {
					return nil, fmt.Errorf("script name cannot be empty")
				}
				config.Scripts[key] = val
			} else {
				return nil, fmt.Errorf("unknown block: '[%s]'", currentBlock)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filepath, err)
	}

	return config, nil
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func parseConfigSetup(setup *models.ConfigSetup, key, val string) error {
	switch key {
	case "compiler":
		setup.Compiler = val
	case "flags":
		setup.Flags = val
	case "name":
		setup.Name = val
	default:
		return fmt.Errorf("unknown variable '%s'", key)
	}
	return nil
}

func parseConfigDependency(dep *models.ConfigDependency, key, val string) error {
	switch key {
	case "target":
		dep.Target = val
	case "sources":
		// split by space for multiple sources
		sources := strings.Fields(val)
		dep.Sources = append(dep.Sources, sources...)
	case "includes":
		includes := strings.Fields(val)
		// Process includes: strip trailing /* if any
		for _, inc := range includes {
			if strings.HasSuffix(inc, "/*") {
				inc = inc[:len(inc)-2]
			}
			dep.Includes = append(dep.Includes, inc)
		}
	case "object.dpdcy":
		valLower := strings.ToLower(val)
		if valLower == "yes" || valLower == "true" {
			dep.ObjectDpdcy = true
		} else if valLower == "no" || valLower == "false" || valLower == "" {
			dep.ObjectDpdcy = false
		} else {
			return fmt.Errorf("invalid value '%s' for object.dpdcy, expected 'yes' or 'no'", val)
		}
	case "build.type":
		valLower := strings.ToLower(val)
		if valLower == "executable" || valLower == "static" || valLower == "shared" || valLower == "" {
			if valLower == "" { dep.BuildType = "executable" } else { dep.BuildType = valLower }
		} else {
			return fmt.Errorf("invalid build.type '%s', expected 'executable', 'static', or 'shared'", val)
		}
	case "libs":
		dep.Libs = val
	default:
		return fmt.Errorf("unknown variable '%s'", key)
	}
	return nil
}
