package converter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Anhdeface/gmake/internal/models"
)

// FormatConfig formats a GomakeConfig model into valid .gomake configuration syntax.
func FormatConfig(cfg *models.GomakeConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("gomake config is nil")
	}

	var sb strings.Builder

	// 1. [const] block
	sb.WriteString("[const]\n")
	if len(cfg.Constants) > 0 {
		sb.WriteString(strings.Join(cfg.Constants, ", "))
		sb.WriteString("\n")
	}
	sb.WriteString("[end]\n\n")

	// 2. Target blocks: [config.setup.<c>] and [config.dependency.<c>]
	for _, c := range cfg.Constants {
		// Setup block
		if setup, exists := cfg.Setups[c]; exists && setup != nil {
			sb.WriteString(fmt.Sprintf("[config.setup.%s]\n", c))
			if setup.Compiler != "" {
				sb.WriteString(fmt.Sprintf("compiler = %s\n", setup.Compiler))
			} else {
				sb.WriteString("compiler = gcc\n")
			}
			if setup.Flags != "" {
				sb.WriteString(fmt.Sprintf("flags = %s\n", setup.Flags))
			}
			if setup.Name != "" {
				sb.WriteString(fmt.Sprintf("name = %s\n", setup.Name))
			} else {
				sb.WriteString(fmt.Sprintf("name = %s\n", c))
			}
			sb.WriteString("[end]\n\n")
		}

		// Dependency block
		if dep, exists := cfg.Dependencies[c]; exists && dep != nil {
			sb.WriteString(fmt.Sprintf("[config.dependency.%s]\n", c))
			if dep.Target != "" {
				sb.WriteString(fmt.Sprintf("target = %s\n", dep.Target))
			}
			if len(dep.Sources) > 0 {
				sb.WriteString(fmt.Sprintf("sources = %s\n", strings.Join(dep.Sources, " ")))
			}
			if len(dep.Includes) > 0 {
				sb.WriteString(fmt.Sprintf("includes = %s\n", strings.Join(dep.Includes, " ")))
			}
			if dep.ObjectDpdcy {
				sb.WriteString("object.dpdcy = yes\n")
			} else {
				sb.WriteString("object.dpdcy = no\n")
			}
			if dep.BuildType != "" {
				sb.WriteString(fmt.Sprintf("build.type = %s\n", dep.BuildType))
			} else {
				sb.WriteString("build.type = executable\n")
			}
			if dep.Libs != "" {
				sb.WriteString(fmt.Sprintf("libs = %s\n", dep.Libs))
			}
			sb.WriteString("[end]\n\n")
		}
	}

	// 3. [config.scripts] block
	if len(cfg.Scripts) > 0 {
		sb.WriteString("[config.scripts]\n")
		var scriptNames []string
		for name := range cfg.Scripts {
			scriptNames = append(scriptNames, name)
		}
		sort.Strings(scriptNames)
		for _, name := range scriptNames {
			sb.WriteString(fmt.Sprintf("%s = %s\n", name, cfg.Scripts[name]))
		}
		sb.WriteString("[end]\n\n")
	}

	// 4. Strict EOF marker
	sb.WriteString("./gomake\n")

	return sb.String(), nil
}

// SerializeConfig is an alias for FormatConfig.
func SerializeConfig(cfg *models.GomakeConfig) (string, error) {
	return FormatConfig(cfg)
}
