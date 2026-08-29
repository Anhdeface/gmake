package converter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Anhdeface/gmake/internal/models"
)

// Converter coordinates translation from MakefileAST to models.GomakeConfig.
type Converter struct {
	ast        *MakefileAST
	usedConsts map[string]int
	config     *models.GomakeConfig
	warnings   []string
}

// NewConverter initializes a new Converter with the provided AST.
func NewConverter(ast *MakefileAST) *Converter {
	return &Converter{
		ast:        ast,
		usedConsts: make(map[string]int),
		config: &models.GomakeConfig{
			Constants:    make([]string, 0),
			Setups:       make(map[string]*models.ConfigSetup),
			Dependencies: make(map[string]*models.ConfigDependency),
			Scripts:      make(map[string]string),
		},
		warnings: make([]string, 0),
	}
}

// ConvertAST translates a MakefileAST into GomakeConfig.
func ConvertAST(ast *MakefileAST) (*models.GomakeConfig, error) {
	c := NewConverter(ast)
	return c.Convert()
}

// Convert performs the full translation pipeline.
func (c *Converter) Convert() (*models.GomakeConfig, error) {
	if c.ast == nil {
		return c.config, nil
	}

	// Track whether pattern rules exist in AST (e.g. %.o: %.c)
	hasObjectPatternRule := len(c.ast.PatternRules) > 0
	for _, r := range c.ast.Rules {
		if r.IsPattern || strings.Contains(r.Target(), "%") {
			hasObjectPatternRule = true
			break
		}
	}

	// 1. Process and classify rules in AST order
	for _, rule := range c.ast.Rules {
		if rule == nil || len(rule.Targets) == 0 {
			continue
		}

		for _, target := range rule.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}

			// Check special targets
			if c.isSpecialTarget(target, rule) {
				continue
			}

			// Check if rule is a build target or script target
			if c.isBuildTarget(target, rule) {
				c.processBuildTarget(target, rule, hasObjectPatternRule)
			} else if c.isScriptTarget(target, rule) {
				c.processScriptTarget(target, rule)
			}
		}
	}

	return c.config, nil
}

// isSpecialTarget identifies standard makefile targets that should be discarded.
func (c *Converter) isSpecialTarget(target string, rule *MakefileRule) bool {
	// Discard dot-prefixed special targets (.PHONY, .SUFFIXES, .DEFAULT_GOAL)
	if strings.HasPrefix(target, ".") {
		return true
	}
	// Discard pattern targets (e.g. %.o)
	if rule.IsPattern || strings.Contains(target, "%") {
		return true
	}
	// Discard intermediate object file rules (e.g. main.o, src/util.o)
	if strings.HasSuffix(target, ".o") || strings.HasSuffix(target, ".obj") {
		return true
	}
	// Discard standard 'all' aggregator rule (gomake synthesizes all: $(TARGETS))
	if target == "all" {
		return true
	}
	// Discard standard 'clean' rule if it only removes binaries/objects
	if target == "clean" {
		return true
	}
	return false
}

// isBuildTarget determines if a rule corresponds to a C/C++ compilation target.
func (c *Converter) isBuildTarget(target string, rule *MakefileRule) bool {
	// If target has recognized library or binary extensions
	ext := strings.ToLower(filepath.Ext(target))
	if ext == ".a" || ext == ".so" || ext == ".dylib" || ext == ".dll" || ext == ".exe" || ext == ".bin" {
		return true
	}

	// If target is in a binary/library path (e.g. bin/app, lib/core)
	dir := filepath.Dir(target)
	if dir == "bin" || dir == "lib" || dir == "build" || dir == "out" || dir == "dist" {
		return true
	}

	// If prerequisites contain source files or object files
	for _, p := range rule.Prerequisites {
		pExt := strings.ToLower(filepath.Ext(p))
		if pExt == ".c" || pExt == ".cpp" || pExt == ".cc" || pExt == ".cxx" || pExt == ".o" || pExt == ".obj" {
			return true
		}
		if strings.Contains(p, "*.c") || strings.Contains(p, "*.cpp") {
			return true
		}
	}

	// If recipe contains compiler or archive tool invocations
	for _, rec := range rule.Recipes {
		recLower := strings.ToLower(rec)
		if strings.Contains(recLower, "$(cc)") || strings.Contains(recLower, "${cc}") ||
			strings.Contains(recLower, "$(cxx)") || strings.Contains(recLower, "${cxx}") ||
			strings.Contains(recLower, "gcc") || strings.Contains(recLower, "g++") ||
			strings.Contains(recLower, "clang") || strings.Contains(recLower, "clang++") ||
			strings.Contains(recLower, "ar rcs") || strings.Contains(recLower, "ar cr") ||
			strings.Contains(recLower, " -o ") {
			return true
		}
	}

	return false
}

// isScriptTarget checks if rule represents a utility script.
func (c *Converter) isScriptTarget(target string, rule *MakefileRule) bool {
	// If rule has no recipes, it is empty/dummy
	if len(rule.Recipes) == 0 {
		return false
	}
	return true
}

// processBuildTarget handles compilation target extraction.
func (c *Converter) processBuildTarget(target string, rule *MakefileRule, hasObjectPatternRule bool) {
	constId := c.normalizeIdentifier(target)

	// 1. Extract Sources and Object Dependency
	sources, objectDpdcy := c.extractSources(rule)
	if hasObjectPatternRule {
		objectDpdcy = true
	}

	// 2. Infer Build Type
	buildType := c.inferBuildType(target, rule)

	// 3. Extract Compiler, Flags, Includes, and Libs
	compiler, flags, includes, libs := c.extractToolchainAndFlags(target, rule, sources)

	// 4. Populate GomakeConfig
	c.config.Constants = append(c.config.Constants, constId)

	c.config.Setups[constId] = &models.ConfigSetup{
		Compiler: compiler,
		Flags:    flags,
		Name:     filepath.Base(target),
	}

	c.config.Dependencies[constId] = &models.ConfigDependency{
		Target:      target,
		Sources:     sources,
		Includes:    includes,
		ObjectDpdcy: objectDpdcy,
		BuildType:   buildType,
		Libs:        libs,
	}
}

// processScriptTarget handles utility script extraction.
func (c *Converter) processScriptTarget(target string, rule *MakefileRule) {
	if len(rule.Recipes) == 0 {
		return
	}

	var commands []string
	for _, rec := range rule.Recipes {
		clean := strings.TrimSpace(rec)
		if clean == "" {
			continue
		}
		// Expand any $(TARGET) or other variable references in script command
		expanded := c.expandScriptCommand(clean, target)
		if expanded != "" {
			commands = append(commands, expanded)
		}
	}

	if len(commands) > 0 {
		c.config.Scripts[target] = strings.Join(commands, " && ")
	}
}

// normalizeIdentifier sanitizes target paths into valid, unique [const] identifiers.
func (c *Converter) normalizeIdentifier(target string) string {
	base := filepath.Base(target)

	// Strip common binary/library extensions
	ext := filepath.Ext(base)
	if ext == ".a" || ext == ".so" || ext == ".dylib" || ext == ".dll" || ext == ".exe" || ext == ".bin" || ext == ".out" || ext == ".o" {
		base = strings.TrimSuffix(base, ext)
	}

	// Convert to lowercase
	base = strings.ToLower(base)

	// Replace non-alphanumeric characters with underscore
	re := regexp.MustCompile(`[^a-z0-9_]+`)
	base = re.ReplaceAllString(base, "_")

	// Strip leading/trailing underscores and collapse multiples
	base = strings.Trim(base, "_")
	reMulti := regexp.MustCompile(`_+`)
	base = reMulti.ReplaceAllString(base, "_")

	if base == "" {
		base = "app"
	}

	// Collision deduplication
	count, exists := c.usedConsts[base]
	if !exists {
		c.usedConsts[base] = 1
		return base
	}

	c.usedConsts[base] = count + 1
	return fmt.Sprintf("%s_%d", base, count+1)
}

// inferBuildType classifies artifact as static, shared, or executable.
func (c *Converter) inferBuildType(target string, rule *MakefileRule) string {
	ext := strings.ToLower(filepath.Ext(target))
	if ext == ".a" {
		return "static"
	}
	if ext == ".so" || ext == ".dylib" || ext == ".dll" {
		return "shared"
	}

	for _, rec := range rule.Recipes {
		recLower := strings.ToLower(rec)
		if strings.Contains(recLower, "ar rcs") || strings.Contains(recLower, "ar cr") {
			return "static"
		}
		if strings.Contains(recLower, "-shared") || strings.Contains(recLower, "-dynamiclib") {
			return "shared"
		}
	}

	return "executable"
}

// extractSources extracts C/C++ source files and detects object dependency.
func (c *Converter) extractSources(rule *MakefileRule) ([]string, bool) {
	var sources []string
	objectDpdcy := false

	// Check prerequisites
	for _, p := range rule.Prerequisites {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pExt := strings.ToLower(filepath.Ext(p))
		if pExt == ".c" || pExt == ".cpp" || pExt == ".cc" || pExt == ".cxx" || pExt == ".s" || pExt == ".asm" || strings.Contains(p, "*") {
			sources = append(sources, p)
		} else if pExt == ".o" || pExt == ".obj" {
			objectDpdcy = true
			// Attempt to map .o to .c or .cpp
			srcCandidate := strings.TrimSuffix(p, pExt) + ".c"
			sources = append(sources, srcCandidate)
		}
	}

	// If no sources from prerequisites, check AST Variables and RawVars
	if len(sources) == 0 {
		targetBase := strings.ToLower(filepath.Base(rule.Target()))
		targetBase = strings.TrimSuffix(targetBase, filepath.Ext(targetBase))
		targetBase = strings.TrimPrefix(targetBase, "lib")

		candidateVarNames := []string{
			"SRCS", "SOURCES", "SRC", "SRC_FILES", "C_SRCS", "CXX_SRCS",
			"SRCS_" + strings.ToUpper(targetBase),
			"SRC_" + strings.ToUpper(targetBase),
		}

		for _, varName := range candidateVarNames {
			// Check expanded variables
			if val, ok := c.ast.Variables[varName]; ok && strings.TrimSpace(val) != "" {
				fields := strings.Fields(val)
				for _, f := range fields {
					fExt := strings.ToLower(filepath.Ext(f))
					if fExt == ".c" || fExt == ".cpp" || fExt == ".cc" || fExt == ".cxx" || strings.Contains(f, "*") {
						sources = append(sources, f)
					}
				}
			}
			// Also check raw vars for wildcard expressions like $(wildcard src/*.c)
			if rawVal, ok := c.ast.RawVars[varName]; ok && strings.TrimSpace(rawVal) != "" {
				if strings.Contains(rawVal, "wildcard") {
					reWildcard := regexp.MustCompile(`\$\((?:wildcard\s+)([^)]+)\)|\$\{(?:wildcard\s+)([^}]+)\}`)
					matches := reWildcard.FindAllStringSubmatch(rawVal, -1)
					for _, m := range matches {
						pattern := strings.TrimSpace(m[1])
						if pattern == "" {
							pattern = strings.TrimSpace(m[2])
						}
						if pattern != "" {
							sources = append(sources, strings.Fields(pattern)...)
						}
					}
				}
			}
			if len(sources) > 0 {
				break
			}
		}
	}

	// Check if OBJS was defined with suffix substitution $(SRCS:.c=.o)
	if objsVal, ok := c.ast.RawVars["OBJS"]; ok && (strings.Contains(objsVal, ".o") || strings.Contains(objsVal, ":.c=.o") || strings.Contains(objsVal, ":.cpp=.o")) {
		objectDpdcy = true
	}
	if objsVal, ok := c.ast.Variables["OBJS"]; ok && strings.Contains(objsVal, ".o") {
		objectDpdcy = true
	}

	// Deduplicate sources while preserving order
	seen := make(map[string]bool)
	var deduped []string
	for _, s := range sources {
		if !seen[s] {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}

	return deduped, objectDpdcy
}

// extractToolchainAndFlags extracts compiler, compilation flags, includes, and libraries.
func (c *Converter) extractToolchainAndFlags(target string, rule *MakefileRule, sources []string) (string, string, []string, string) {
	isCpp := false
	for _, s := range sources {
		sExt := strings.ToLower(filepath.Ext(s))
		if sExt == ".cpp" || sExt == ".cc" || sExt == ".cxx" || sExt == ".c++" || sExt == ".C" {
			isCpp = true
			break
		}
	}

	// 1. Determine Compiler
	compiler := ""
	for _, rec := range rule.Recipes {
		recLower := strings.ToLower(rec)
		if strings.Contains(recLower, "g++") || strings.Contains(recLower, "$(cxx)") || strings.Contains(recLower, "${cxx}") {
			isCpp = true
			compiler = "g++"
			break
		} else if strings.Contains(recLower, "clang++") {
			isCpp = true
			compiler = "clang++"
			break
		} else if strings.Contains(recLower, "clang") {
			compiler = "clang"
			break
		} else if strings.Contains(recLower, "gcc") || strings.Contains(recLower, "$(cc)") || strings.Contains(recLower, "${cc}") {
			compiler = "gcc"
			break
		}
	}

	if compiler == "" {
		if isCpp {
			if cxx, ok := c.ast.Variables["CXX"]; ok && cxx != "" {
				compiler = cxx
			} else {
				compiler = "g++"
			}
		} else {
			if cc, ok := c.ast.Variables["CC"]; ok && cc != "" {
				compiler = cc
			} else {
				compiler = "gcc"
			}
		}
	}

	// 2. Aggregate Raw Flags & Variables
	var rawFlagsParts []string
	if isCpp {
		if val, ok := c.ast.Variables["CXXFLAGS"]; ok && val != "" {
			rawFlagsParts = append(rawFlagsParts, val)
		}
	} else {
		if val, ok := c.ast.Variables["CFLAGS"]; ok && val != "" {
			rawFlagsParts = append(rawFlagsParts, val)
		}
	}
	if val, ok := c.ast.Variables["CPPFLAGS"]; ok && val != "" {
		rawFlagsParts = append(rawFlagsParts, val)
	}

	// Also extract inline flags from recipes (excluding mkdir commands)
	for _, rec := range rule.Recipes {
		recTrim := strings.TrimSpace(rec)
		if strings.HasPrefix(recTrim, "mkdir") || strings.HasPrefix(recTrim, "@mkdir") {
			continue
		}
		rawFlagsParts = append(rawFlagsParts, rec)
	}

	rawFlags := strings.Join(rawFlagsParts, " ")

	// 3. Dissect Flags into Clean Flags, Includes, and Libs
	cleanFlags, includes, libs := c.dissectFlags(rawFlags, target, compiler, sources)

	// Merge global INCLUDES variable if defined
	if globalInc, ok := c.ast.Variables["INCLUDES"]; ok && globalInc != "" {
		for _, inc := range strings.Fields(globalInc) {
			cleanInc := strings.TrimPrefix(inc, "-I")
			cleanInc = strings.TrimSuffix(cleanInc, "/*")
			cleanInc = strings.Trim(cleanInc, `"'`)
			cleanInc = strings.TrimSpace(cleanInc)
			if cleanInc != "" && !containsStr(includes, cleanInc) {
				includes = append(includes, cleanInc)
			}
		}
	}

	// Merge global LIBS, LDFLAGS, LDLIBS variables if defined
	var globalLibsParts []string
	for _, varName := range []string{"LIBS", "LDFLAGS", "LDLIBS"} {
		if val, ok := c.ast.Variables[varName]; ok && val != "" {
			globalLibsParts = append(globalLibsParts, val)
		}
	}
	if len(globalLibsParts) > 0 {
		globalLibs := strings.Join(globalLibsParts, " ")
		for _, libToken := range strings.Fields(globalLibs) {
			if libToken != "" && !strings.Contains(libs, libToken) {
				if libs == "" {
					libs = libToken
				} else {
					libs = libs + " " + libToken
				}
			}
		}
	}

	return compiler, cleanFlags, includes, libs
}

// dissectFlags parses tokens to extract includes (-I), libs (-l, -L), and compilation flags.
func (c *Converter) dissectFlags(raw string, target, compiler string, sources []string) (string, []string, string) {
	tokens := strings.Fields(raw)
	var flagTokens []string
	var includes []string
	var libTokens []string

	sourceMap := make(map[string]bool)
	for _, s := range sources {
		sourceMap[s] = true
		sourceMap[filepath.Base(s)] = true
	}

	skipNext := false
	for i := 0; i < len(tokens); i++ {
		if skipNext {
			skipNext = false
			continue
		}

		tok := tokens[i]

		// Skip recipe prefixes, compiler names, target outputs
		if tok == compiler || tok == "$(CC)" || tok == "${CC}" || tok == "$(CXX)" || tok == "${CXX}" ||
			tok == "gcc" || tok == "g++" || tok == "clang" || tok == "clang++" || tok == "ar" || tok == "rcs" || tok == "cr" ||
			tok == "$@" || tok == "$<" || tok == "$^" || tok == "$*" || tok == "-o" || tok == "-c" ||
			tok == "-shared" || tok == "-dynamiclib" || tok == target || tok == "$(SRCS)" || tok == "${SRCS}" ||
			tok == "$(OBJS)" || tok == "${OBJS}" || tok == "$(INCLUDES)" || tok == "${INCLUDES}" ||
			tok == "$(LIBS)" || tok == "${LIBS}" || tok == "$(LDFLAGS)" || tok == "${LDFLAGS}" ||
			tok == "$(CFLAGS)" || tok == "${CFLAGS}" || tok == "$(CXXFLAGS)" || tok == "${CXXFLAGS}" ||
			sourceMap[tok] {
			if tok == "-o" && i+1 < len(tokens) {
				skipNext = true
			}
			continue
		}

		// Handle Include flags: -I<dir> or -I <dir>
		if strings.HasPrefix(tok, "-I") {
			incPath := strings.TrimPrefix(tok, "-I")
			if incPath == "" && i+1 < len(tokens) {
				incPath = tokens[i+1]
				skipNext = true
			}
			incPath = strings.TrimSuffix(incPath, "/*")
			incPath = strings.Trim(incPath, `"'`)
			if incPath != "" && !containsStr(includes, incPath) {
				includes = append(includes, incPath)
			}
			continue
		}

		// Handle Lib flags: -l<name>, -L<dir>, -Wl,...
		if strings.HasPrefix(tok, "-l") || strings.HasPrefix(tok, "-L") || strings.HasPrefix(tok, "-Wl,") {
			if !containsStr(libTokens, tok) {
				libTokens = append(libTokens, tok)
			}
			continue
		}

		// Handle standard compiler flags (-Wall, -O2, -g, -std=..., -D..., -fPIC, -pedantic, etc.)
		if strings.HasPrefix(tok, "-") {
			if !containsStr(flagTokens, tok) {
				flagTokens = append(flagTokens, tok)
			}
		}
	}

	return strings.Join(flagTokens, " "), includes, strings.Join(libTokens, " ")
}

// expandScriptCommand resolves variable references in script commands.
func (c *Converter) expandScriptCommand(cmd, target string) string {
	// Strip leading echo suppression or error ignoring modifiers
	cmd = strings.TrimLeft(cmd, "@-+")
	cmd = strings.TrimSpace(cmd)

	// Replace automatic variables
	cmd = strings.ReplaceAll(cmd, "$@", target)
	cmd = strings.ReplaceAll(cmd, "$<", target)

	// Replace AST variables
	for k, v := range c.ast.Variables {
		cmd = strings.ReplaceAll(cmd, fmt.Sprintf("$(%s)", k), v)
		cmd = strings.ReplaceAll(cmd, fmt.Sprintf("${%s}", k), v)
	}

	return cmd
}

func containsStr(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
