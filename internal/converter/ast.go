package converter

import "strings"

// AssignmentFlavor defines the variable assignment operator flavor.
type AssignmentFlavor string

const (
	FlavorRecursive   AssignmentFlavor = "="   // Lazily expanded variable
	FlavorSimple      AssignmentFlavor = ":="  // Immediately expanded variable
	FlavorSimplePOSIX AssignmentFlavor = "::=" // POSIX standard immediately expanded variable
	FlavorConditional AssignmentFlavor = "?="  // Set only if variable not already defined
	FlavorAppend      AssignmentFlavor = "+="  // Append value with space separator
	FlavorShell       AssignmentFlavor = "!="  // Shell command output assignment
)

// VariableAssignment represents a parsed variable definition.
type VariableAssignment struct {
	Name       string           // Variable identifier (e.g. "CC", "CFLAGS", "SRCS")
	Value      string           // Evaluated or assigned value
	RawValue   string           // Raw unexpanded value
	Flavor     AssignmentFlavor // Operator flavor (=, :=, ::=, ?=, +=, !=)
	Target     string           // Target name if target-specific (e.g. "app: CFLAGS += -g")
	Exported   bool             // True if declared with "export"
	Override   bool             // True if declared with "override"
	LineNumber int              // Starting 1-based physical line number
}

// DirectiveType defines recognized Makefile directives.
type DirectiveType string

const (
	DirectiveInclude         DirectiveType = "include"  // include <files>
	DirectiveOptionalInclude DirectiveType = "-include" // -include <files> (ignore missing)
	DirectiveSInclude        DirectiveType = "sinclude" // sinclude <files> (synonym for -include)
	DirectiveIfEq            DirectiveType = "ifeq"     // ifeq (arg1, arg2)
	DirectiveIfNeq           DirectiveType = "ifneq"    // ifneq (arg1, arg2)
	DirectiveIfDef           DirectiveType = "ifdef"    // ifdef <var>
	DirectiveIfNDef          DirectiveType = "ifndef"   // ifndef <var>
	DirectiveElse            DirectiveType = "else"     // else / else ifeq (...)
	DirectiveEndIf           DirectiveType = "endif"    // endif
	DirectiveExport          DirectiveType = "export"   // export <vars>
	DirectiveUnexport        DirectiveType = "unexport" // unexport <vars>
	DirectiveOverride        DirectiveType = "override" // override <var> = <val>
	DirectiveVPath           DirectiveType = "vpath"    // vpath <pattern> <directories>
	DirectiveUnknown         DirectiveType = "unknown"  // Unrecognized directive
)

// Directive represents a Makefile directive construct.
type Directive struct {
	Type       DirectiveType // Directive keyword type
	Args       string        // Unparsed arguments following directive keyword
	LineNumber int           // Starting 1-based physical line number
}

// DiagnosticSeverity defines logging severity levels.
type DiagnosticSeverity string

const (
	SeverityInfo  DiagnosticSeverity = "info"
	SeverityWarn  DiagnosticSeverity = "warn"
	SeverityError DiagnosticSeverity = "error"
)

// DiagnosticWarning records non-fatal warnings or recoverable syntax issues.
type DiagnosticWarning struct {
	LineNumber int                // Source line number
	Message    string             // Warning description
	Severity   DiagnosticSeverity // Severity level (info, warn, error)
}

// RecipeLine represents a detailed recipe command line with Make execution modifiers.
type RecipeLine struct {
	Raw         string // Original unparsed line
	Command     string // Command with indentation & prefixes stripped
	Silent      bool   // True if prefixed with '@' (suppress echoing)
	IgnoreError bool   // True if prefixed with '-' (ignore exit code)
	AlwaysExec  bool   // True if prefixed with '+' (execute even with make -n)
	LineNumber  int    // Source line number
}

// MakefileRule represents a target rule in the Makefile AST.
type MakefileRule struct {
	Targets       []string     // Target names (e.g. ["app", "bin/app"])
	Prerequisites []string     // Prerequisites/dependencies (e.g. ["main.o", "util.o"])
	OrderOnly     []string     // Order-only prerequisites (| prereqs)
	Recipes       []string     // Shell command recipe lines (clean commands)
	RecipeLines   []RecipeLine // Detailed recipe metadata including prefixes
	IsPhony       bool         // True if listed in .PHONY or identified as phony
	IsPattern     bool         // True if target contains pattern '%' (e.g. %.o: %.c)
	IsDoubleColon bool         // True if defined using '::' rule syntax
	LineNumber    int          // Starting 1-based physical line number
}

// Target returns the primary (first) target name or empty string.
func (r *MakefileRule) Target() string {
	if len(r.Targets) > 0 {
		return r.Targets[0]
	}
	return ""
}

// MakefileAST represents the parsed intermediate representation of a Makefile.
type MakefileAST struct {
	Variables     map[string]string        // Expanded variable lookup table (key -> evaluated value)
	RawVars       map[string]string        // Raw unexpanded variable table (key -> raw string)
	Rules         []*MakefileRule          // Ordered list of all parsed target rules
	RuleMap       map[string]*MakefileRule // Map of target name -> primary rule for fast lookup
	PhonyTargets  map[string]bool          // Set of explicitly declared .PHONY targets
	PatternRules  []*MakefileRule          // Filtered list of pattern rules (e.g. %.o: %.c)
	Directives    []*Directive             // List of parsed directives
	Diagnostics   []DiagnosticWarning      // Collected recoverable warnings during parsing
	Warnings      []string                 // Flat string list of warnings for easy inspection
	IncludedFiles []string                 // Files referenced by include directives
	DefaultGoal   string                   // First non-pattern, non-dot target (default build goal)
}

// NewMakefileAST initializes and returns a ready-to-use MakefileAST.
func NewMakefileAST() *MakefileAST {
	return &MakefileAST{
		Variables:     make(map[string]string),
		RawVars:       make(map[string]string),
		Rules:         make([]*MakefileRule, 0),
		RuleMap:       make(map[string]*MakefileRule),
		PhonyTargets:  make(map[string]bool),
		PatternRules:  make([]*MakefileRule, 0),
		Directives:    make([]*Directive, 0),
		Diagnostics:   make([]DiagnosticWarning, 0),
		Warnings:      make([]string, 0),
		IncludedFiles: make([]string, 0),
	}
}

// AddRule registers a target rule in the AST, updating RuleMap, PatternRules, and DefaultGoal.
func (ast *MakefileAST) AddRule(rule *MakefileRule) {
	if rule == nil {
		return
	}
	ast.Rules = append(ast.Rules, rule)
	if rule.IsPattern {
		ast.PatternRules = append(ast.PatternRules, rule)
	}
	for _, target := range rule.Targets {
		if ast.RuleMap[target] == nil {
			ast.RuleMap[target] = rule
		}
		// Set DefaultGoal if first non-pattern, non-phony, non-dot target
		if ast.DefaultGoal == "" && !rule.IsPattern && !ast.PhonyTargets[target] && !strings.HasPrefix(target, ".") {
			ast.DefaultGoal = target
		}
	}
}

// GetRule returns the rule associated with target name, if present.
func (ast *MakefileAST) GetRule(target string) (*MakefileRule, bool) {
	rule, ok := ast.RuleMap[target]
	return rule, ok
}

// AddVariable registers a variable definition in the AST symbol tables.
func (ast *MakefileAST) AddVariable(assign VariableAssignment) {
	if assign.RawValue == "" && assign.Value != "" {
		assign.RawValue = assign.Value
	}
	ast.RawVars[assign.Name] = assign.RawValue
	ast.Variables[assign.Name] = assign.Value
}

// GetVariable returns the evaluated value of a variable, if defined.
func (ast *MakefileAST) GetVariable(name string) (string, bool) {
	val, ok := ast.Variables[name]
	return val, ok
}

// MarkPhony records targets as phony and marks matching rules.
func (ast *MakefileAST) MarkPhony(targets ...string) {
	for _, t := range targets {
		ast.PhonyTargets[t] = true
		if rule, ok := ast.RuleMap[t]; ok {
			rule.IsPhony = true
		}
	}
}

// AddDiagnostic logs a recoverable warning or informational event.
func (ast *MakefileAST) AddDiagnostic(line int, msg string, sev DiagnosticSeverity) {
	ast.Diagnostics = append(ast.Diagnostics, DiagnosticWarning{
		LineNumber: line,
		Message:    msg,
		Severity:   sev,
	})
	ast.Warnings = append(ast.Warnings, msg)
}
