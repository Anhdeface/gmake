package converter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VarFlavor distinguishes between recursively expanded (=) and simply expanded (:=) variables.
type VarFlavor int

const (
	VarFlavorRecursive VarFlavor = iota // Evaluated lazily upon reference
	VarFlavorSimple                     // Evaluated immediately upon assignment
)

// VarAssignOp represents the variable assignment operator.
type VarAssignOp string

const (
	OpRecursive VarAssignOp = "="
	OpSimple    VarAssignOp = ":="
	OpPOSIX     VarAssignOp = "::="
	OpDefault   VarAssignOp = "?="
	OpAppend    VarAssignOp = "+="
	OpShell     VarAssignOp = "!="
)

// Variable represents a stored Makefile variable definition.
type Variable struct {
	Name     string
	RawValue string
	Value    string // Evaluated value (cached or simple)
	Flavor   VarFlavor
	Origin   string // "default", "file", "environment", "automatic", "target"
	Line     int
}

// AutoVars holds rule-context automatic variables.
type AutoVars struct {
	Target         string   // $@
	FirstPrereq    string   // $<
	Prereqs        []string // $^ (deduplicated), $+ (all)
	Stem           string   // $*
	ArchiveMember  string   // $%
	UpdatedPrereqs []string // $?
}

// Lookup resolves automatic variables and their directory/file variants.
func (a *AutoVars) Lookup(name string) (string, bool) {
	if a == nil {
		return "", false
	}
	switch name {
	case "@": // Target
		return a.Target, true
	case "<": // First prerequisite
		return a.FirstPrereq, true
	case "^": // Deduplicated prerequisites
		seen := make(map[string]bool)
		var deduped []string
		for _, p := range a.Prereqs {
			if !seen[p] {
				seen[p] = true
				deduped = append(deduped, p)
			}
		}
		return strings.Join(deduped, " "), true
	case "+": // All prerequisites in order
		return strings.Join(a.Prereqs, " "), true
	case "*": // Stem
		return a.Stem, true
	case "%": // Archive member
		return a.ArchiveMember, true
	case "?": // Changed prerequisites
		return strings.Join(a.UpdatedPrereqs, " "), true

	// Directory / File Variants
	case "@D":
		d := filepath.Dir(a.Target)
		if d == "." && !strings.HasPrefix(a.Target, ".") {
			return ".", true
		}
		return d, true
	case "@F":
		return filepath.Base(a.Target), true
	case "<D":
		if a.FirstPrereq == "" {
			return "", true
		}
		return filepath.Dir(a.FirstPrereq), true
	case "<F":
		if a.FirstPrereq == "" {
			return "", true
		}
		return filepath.Base(a.FirstPrereq), true
	case "^D":
		seen := make(map[string]bool)
		var dirs []string
		for _, p := range a.Prereqs {
			d := filepath.Dir(p)
			if !seen[d] {
				seen[d] = true
				dirs = append(dirs, d)
			}
		}
		return strings.Join(dirs, " "), true
	case "^F":
		seen := make(map[string]bool)
		var bases []string
		for _, p := range a.Prereqs {
			b := filepath.Base(p)
			if !seen[b] {
				seen[b] = true
				bases = append(bases, b)
			}
		}
		return strings.Join(bases, " "), true
	case "*D":
		return filepath.Dir(a.Stem), true
	case "*F":
		return filepath.Base(a.Stem), true
	}

	return "", false
}

// VarTable manages scoped variable symbol tables with inheritance and defaults.
type VarTable struct {
	parent *VarTable
	vars   map[string]*Variable
}

// NewVarTable initializes a root symbol table populated with standard defaults.
func NewVarTable() *VarTable {
	vt := &VarTable{
		vars: make(map[string]*Variable),
	}
	vt.populateDefaults()
	return vt
}

// NewChildVarTable creates a child scope inheriting from a parent symbol table.
func NewChildVarTable(parent *VarTable) *VarTable {
	return &VarTable{
		parent: parent,
		vars:   make(map[string]*Variable),
	}
}

func (vt *VarTable) populateDefaults() {
	defaults := map[string]string{
		"CC":            "gcc",
		"CXX":           "g++",
		"AR":            "ar",
		"ARFLAGS":       "rcs",
		"RM":            "rm -f",
		"CFLAGS":        "",
		"CXXFLAGS":      "",
		"CPPFLAGS":      "",
		"LDFLAGS":       "",
		"LDLIBS":        "",
		"MAKE":          "gomake",
		".DEFAULT_GOAL": "",
	}
	for k, v := range defaults {
		vt.vars[k] = &Variable{
			Name:     k,
			RawValue: v,
			Value:    v,
			Flavor:   VarFlavorRecursive,
			Origin:   "default",
		}
	}
}

// Get retrieves a variable by name from current or parent scopes.
func (vt *VarTable) Get(name string) (*Variable, bool) {
	if v, ok := vt.vars[name]; ok {
		return v, true
	}
	if vt.parent != nil {
		return vt.parent.Get(name)
	}
	return nil, false
}

// Set sets or modifies a variable according to its assignment operator.
func (vt *VarTable) Set(name, rawVal string, op VarAssignOp, expander *Expander, line int) {

	name = strings.TrimSpace(name)
	existing, exists := vt.vars[name]
	if !exists && vt.parent != nil {
		existing, exists = vt.parent.Get(name)
	}

	switch op {
	case OpSimple, OpPOSIX: // :=, ::=
		expanded := ""
		if expander != nil {
			expanded = expander.Expand(rawVal, nil)
		} else {
			expanded = rawVal
		}
		vt.vars[name] = &Variable{
			Name:     name,
			RawValue: rawVal,
			Value:    expanded,
			Flavor:   VarFlavorSimple,
			Origin:   "file",
			Line:     line,
		}

	case OpRecursive: // =
		vt.vars[name] = &Variable{
			Name:     name,
			RawValue: rawVal,
			Value:    "",
			Flavor:   VarFlavorRecursive,
			Origin:   "file",
			Line:     line,
		}

	case OpDefault: // ?=
		if !exists || existing.Origin == "default" {
			vt.vars[name] = &Variable{
				Name:     name,
				RawValue: rawVal,
				Value:    "",
				Flavor:   VarFlavorRecursive,
				Origin:   "file",
				Line:     line,
			}
		}

	case OpAppend: // +=
		if !exists {
			vt.vars[name] = &Variable{
				Name:     name,
				RawValue: rawVal,
				Value:    "",
				Flavor:   VarFlavorRecursive,
				Origin:   "file",
				Line:     line,
			}
		} else if existing.Flavor == VarFlavorSimple {
			expanded := ""
			if expander != nil {
				expanded = expander.Expand(rawVal, nil)
			} else {
				expanded = rawVal
			}
			newVal := existing.Value
			if newVal != "" && expanded != "" {
				newVal += " " + expanded
			} else if expanded != "" {
				newVal = expanded
			}
			vt.vars[name] = &Variable{
				Name:     name,
				RawValue: existing.RawValue + " " + rawVal,
				Value:    newVal,
				Flavor:   VarFlavorSimple,
				Origin:   "file",
				Line:     line,
			}
		} else { // VarFlavorRecursive
			newRaw := existing.RawValue
			if newRaw != "" && rawVal != "" {
				newRaw += " " + rawVal
			} else if rawVal != "" {
				newRaw = rawVal
			}
			vt.vars[name] = &Variable{
				Name:     name,
				RawValue: newRaw,
				Value:    "",
				Flavor:   VarFlavorRecursive,
				Origin:   "file",
				Line:     line,
			}
		}

	case OpShell: // !=
		cmd := rawVal
		if expander != nil {
			cmd = expander.Expand(rawVal, nil)
		}
		out := executeShellCommand(cmd, expander)
		vt.vars[name] = &Variable{
			Name:     name,
			RawValue: rawVal,
			Value:    out,
			Flavor:   VarFlavorSimple,
			Origin:   "file",
			Line:     line,
		}
	}
}

// ToMap exports all variables fully expanded as a key-value map.
func (vt *VarTable) ToMap(expander *Expander) map[string]string {
	result := make(map[string]string)
	for k := range vt.vars {
		if expander != nil {
			result[k] = expander.Expand("$("+k+")", nil)
		} else {
			result[k] = vt.vars[k].Value
		}
	}
	return result
}

// ToRawMap exports raw unexpanded variable strings.
func (vt *VarTable) ToRawMap() map[string]string {
	result := make(map[string]string)
	for k, v := range vt.vars {
		result[k] = v.RawValue
	}
	return result
}

// Expander coordinates variable dereferencing, nested expansion, and function execution.
type Expander struct {
	Scope    *VarTable
	MaxDepth int
	ShellFn  func(cmd string) (string, error)
	WarnFn   func(format string, args ...any)
}

// NewExpander creates a new variable expander.
func NewExpander(scope *VarTable) *Expander {
	return &Expander{
		Scope:    scope,
		MaxDepth: 64,
	}
}

// Expand performs full recursive expansion of input text.
func (e *Expander) Expand(input string, autoVars *AutoVars) string {
	visited := make(map[string]bool)
	return e.expandInternal(input, autoVars, visited, 0)
}

func (e *Expander) expandInternal(input string, autoVars *AutoVars, visited map[string]bool, depth int) string {
	if depth > e.MaxDepth {
		if e.WarnFn != nil {
			e.WarnFn("expansion depth limit (%d) exceeded; possible circular dependency", e.MaxDepth)
		}
		return ""
	}

	if !strings.ContainsRune(input, '$') {
		return input
	}

	var buf bytes.Buffer
	n := len(input)
	i := 0

	for i < n {
		if input[i] != '$' {
			buf.WriteByte(input[i])
			i++
			continue
		}

		if i+1 >= n {
			buf.WriteByte('$')
			i++
			break
		}

		next := input[i+1]

		// 1. Escaped Dollar: $$ -> $
		if next == '$' {
			buf.WriteByte('$')
			i += 2
			continue
		}

		// 2. Delimited Reference: $(...) or ${...}
		if next == '(' || next == '{' {
			openChar := next
			closeChar := byte(')')
			if openChar == '{' {
				closeChar = '}'
			}

			depthCount := 1
			j := i + 2
			for j < n && depthCount > 0 {
				if input[j] == openChar {
					depthCount++
				} else if input[j] == closeChar {
					depthCount--
				}
				if depthCount == 0 {
					break
				}
				j++
			}

			var inner string
			if depthCount == 0 {
				inner = input[i+2 : j]
				i = j + 1
			} else {
				inner = input[i+2:]
				i = n
			}

			// Case A: Function call $(func arg1,arg2...)
			if isFunctionCall(inner) {
				funcName, argsRaw := splitFunctionCall(inner)
				val := e.evalFunction(funcName, argsRaw, autoVars, visited, depth+1)
				buf.WriteString(val)
				continue
			}

			// Case B: Substitution reference $(VAR:suffix=rep) or $(VAR:%.c=%.o)
			if colonIdx := findTopLevelColon(inner); colonIdx != -1 {
				varPart := inner[:colonIdx]
				substPart := inner[colonIdx+1:]
				if eqIdx := strings.IndexByte(substPart, '='); eqIdx != -1 {
					fromPat := substPart[:eqIdx]
					toPat := substPart[eqIdx+1:]

					fromPat = e.expandInternal(fromPat, autoVars, visited, depth+1)
					toPat = e.expandInternal(toPat, autoVars, visited, depth+1)
					varName := e.expandInternal(varPart, autoVars, visited, depth+1)
					varVal := e.getVarValue(strings.TrimSpace(varName), autoVars, visited, depth+1)

					buf.WriteString(applyPatternOrSuffixSubst(varVal, fromPat, toPat))
					continue
				}
			}

			// Case C: Regular / Nested variable reference $(VAR) or $($(VAR))
			varName := e.expandInternal(inner, autoVars, visited, depth+1)
			varVal := e.getVarValue(strings.TrimSpace(varName), autoVars, visited, depth+1)
			buf.WriteString(varVal)
			continue
		}

		// 3. Single-character reference: $@, $<, $^, $*, $+ etc.
		charName := string(next)
		varVal := e.getVarValue(charName, autoVars, visited, depth+1)
		buf.WriteString(varVal)
		i += 2
	}

	return buf.String()
}

func (e *Expander) getVarValue(name string, autoVars *AutoVars, visited map[string]bool, depth int) string {
	// 1. Check contextual automatic variables
	if autoVars != nil {
		if val, ok := autoVars.Lookup(name); ok {
			return val
		}
	}

	// 2. Check symbol table
	if e.Scope != nil {
		if v, ok := e.Scope.Get(name); ok {
			if v.Flavor == VarFlavorSimple {
				return v.Value
			}
			// Recursive variable: check cycle
			if visited[name] {
				if e.WarnFn != nil {
					e.WarnFn("recursive variable cycle detected for $(%s)", name)
				}
				return ""
			}
			visited[name] = true
			res := e.expandInternal(v.RawValue, autoVars, visited, depth+1)
			delete(visited, name)
			return res
		}
	}

	// 3. Check environment
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return ""
}

func applyPatternOrSuffixSubst(text, from, to string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var results []string
	if strings.ContainsRune(from, '%') {
		fromPrefix, fromSuffix, _ := splitPattern(from)
		toPrefix, toSuffix, hasToPercent := splitPattern(to)

		for _, w := range words {
			if strings.HasPrefix(w, fromPrefix) && strings.HasSuffix(w, fromSuffix) &&
				len(w) >= len(fromPrefix)+len(fromSuffix) {
				stem := w[len(fromPrefix) : len(w)-len(fromSuffix)]
				if hasToPercent {
					results = append(results, toPrefix+stem+toSuffix)
				} else {
					results = append(results, to)
				}
			} else {
				results = append(results, w)
			}
		}
	} else {
		for _, w := range words {
			if strings.HasSuffix(w, from) {
				stem := strings.TrimSuffix(w, from)
				results = append(results, stem+to)
			} else {
				results = append(results, w)
			}
		}
	}

	var cleanResults []string
	for _, r := range results {
		if r != "" {
			cleanResults = append(cleanResults, r)
		}
	}
	return strings.Join(cleanResults, " ")
}

func splitPattern(pattern string) (prefix, suffix string, hasPercent bool) {
	idx := strings.IndexByte(pattern, '%')
	if idx == -1 {
		return pattern, "", false
	}
	return pattern[:idx], pattern[idx+1:], true
}

func isFunctionCall(inner string) bool {
	firstWord, rest := splitFirstWord(inner)
	if rest == "" && !strings.ContainsAny(inner, " \t,") {
		return false
	}
	switch firstWord {
	case "patsubst", "subst", "wildcard", "addprefix", "addsuffix",
		"dir", "notdir", "basename", "suffix", "filter", "filter-out",
		"strip", "firstword", "lastword", "word", "words", "wordlist",
		"sort", "join", "if", "or", "and", "foreach", "call", "shell",
		"eval", "info", "warning", "error", "origin", "flavor":
		return true
	}
	return false
}

func splitFirstWord(s string) (string, string) {
	trimmed := strings.TrimLeft(s, " \t")
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == ' ' || trimmed[i] == '\t' || trimmed[i] == '\n' {
			return trimmed[:i], strings.TrimLeft(trimmed[i:], " \t")
		}
	}
	return trimmed, ""
}

func splitFunctionCall(inner string) (string, string) {
	return splitFirstWord(inner)
}

func findTopLevelColon(inner string) int {
	depthParen := 0
	depthBrace := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case ':':
			if depthParen == 0 && depthBrace == 0 {
				return i
			}
		}
	}
	return -1
}

func splitFuncArgs(argsRaw string) []string {
	var args []string
	var current bytes.Buffer
	depthParen := 0
	depthBrace := 0

	for i := 0; i < len(argsRaw); i++ {
		ch := argsRaw[i]
		switch ch {
		case '(':
			depthParen++
			current.WriteByte(ch)
		case ')':
			if depthParen > 0 {
				depthParen--
			}
			current.WriteByte(ch)
		case '{':
			depthBrace++
			current.WriteByte(ch)
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
			current.WriteByte(ch)
		case ',':
			if depthParen == 0 && depthBrace == 0 {
				args = append(args, current.String())
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}
	args = append(args, current.String())
	return args
}

func (e *Expander) evalFunction(funcName, argsRaw string, autoVars *AutoVars, visited map[string]bool, depth int) string {
	switch funcName {
	case "patsubst":
		args := splitFuncArgs(argsRaw)
		if len(args) < 3 {
			return ""
		}
		pattern := strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1))
		replacement := strings.TrimSpace(e.expandInternal(args[1], autoVars, visited, depth+1))
		text := e.expandInternal(args[2], autoVars, visited, depth+1)
		return applyPatternOrSuffixSubst(text, pattern, replacement)

	case "subst":
		args := splitFuncArgs(argsRaw)
		if len(args) < 3 {
			return ""
		}
		from := e.expandInternal(args[0], autoVars, visited, depth+1)
		to := e.expandInternal(args[1], autoVars, visited, depth+1)
		text := strings.TrimSpace(e.expandInternal(args[2], autoVars, visited, depth+1))
		return strings.ReplaceAll(text, from, to)

	case "wildcard":
		expandedPattern := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		pats := strings.Fields(expandedPattern)
		var matches []string
		for _, p := range pats {
			m, err := filepath.Glob(p)
			if err == nil && len(m) > 0 {
				matches = append(matches, m...)
			}
		}
		if len(matches) == 0 {
			return expandedPattern
		}
		return strings.Join(matches, " ")

	case "addprefix":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		prefix := e.expandInternal(args[0], autoVars, visited, depth+1)
		text := e.expandInternal(args[1], autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			results = append(results, prefix+w)
		}
		return strings.Join(results, " ")

	case "addsuffix":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		suffix := e.expandInternal(args[0], autoVars, visited, depth+1)
		text := e.expandInternal(args[1], autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			results = append(results, w+suffix)
		}
		return strings.Join(results, " ")

	case "dir":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			idx := strings.LastIndexByte(w, '/')
			if idx == -1 {
				results = append(results, "./")
			} else {
				results = append(results, w[:idx+1])
			}
		}
		return strings.Join(results, " ")

	case "notdir":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			idx := strings.LastIndexByte(w, '/')
			if idx == -1 {
				results = append(results, w)
			} else {
				results = append(results, w[idx+1:])
			}
		}
		return strings.Join(results, " ")

	case "basename":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			slashIdx := strings.LastIndexByte(w, '/')
			dotIdx := strings.LastIndexByte(w, '.')
			if dotIdx > slashIdx {
				results = append(results, w[:dotIdx])
			} else {
				results = append(results, w)
			}
		}
		return strings.Join(results, " ")

	case "suffix":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			slashIdx := strings.LastIndexByte(w, '/')
			dotIdx := strings.LastIndexByte(w, '.')
			if dotIdx > slashIdx {
				results = append(results, w[dotIdx:])
			}
		}
		return strings.Join(results, " ")

	case "filter":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		patterns := strings.Fields(e.expandInternal(args[0], autoVars, visited, depth+1))
		text := e.expandInternal(args[1], autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			for _, pat := range patterns {
				if matchPattern(pat, w) {
					results = append(results, w)
					break
				}
			}
		}
		return strings.Join(results, " ")

	case "filter-out":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		patterns := strings.Fields(e.expandInternal(args[0], autoVars, visited, depth+1))
		text := e.expandInternal(args[1], autoVars, visited, depth+1)
		words := strings.Fields(text)
		var results []string
		for _, w := range words {
			matched := false
			for _, pat := range patterns {
				if matchPattern(pat, w) {
					matched = true
					break
				}
			}
			if !matched {
				results = append(results, w)
			}
		}
		return strings.Join(results, " ")

	case "strip":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		return strings.Join(strings.Fields(text), " ")

	case "firstword":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		if len(words) > 0 {
			return words[0]
		}
		return ""

	case "lastword":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		if len(words) > 0 {
			return words[len(words)-1]
		}
		return ""

	case "word":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		nStr := strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1))
		n, err := strconv.Atoi(nStr)
		if err != nil || n < 1 {
			return ""
		}
		words := strings.Fields(e.expandInternal(args[1], autoVars, visited, depth+1))
		if n <= len(words) {
			return words[n-1]
		}
		return ""

	case "words":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		return strconv.Itoa(len(strings.Fields(text)))

	case "wordlist":
		args := splitFuncArgs(argsRaw)
		if len(args) < 3 {
			return ""
		}
		s, _ := strconv.Atoi(strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1)))
		end, _ := strconv.Atoi(strings.TrimSpace(e.expandInternal(args[1], autoVars, visited, depth+1)))
		words := strings.Fields(e.expandInternal(args[2], autoVars, visited, depth+1))
		if s < 1 {
			s = 1
		}
		if end > len(words) {
			end = len(words)
		}
		if s > end || s > len(words) {
			return ""
		}
		return strings.Join(words[s-1:end], " ")

	case "sort":
		text := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		words := strings.Fields(text)
		sort.Strings(words)
		var unique []string
		for i, w := range words {
			if i == 0 || w != words[i-1] {
				unique = append(unique, w)
			}
		}
		return strings.Join(unique, " ")

	case "join":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		l1 := strings.Fields(e.expandInternal(args[0], autoVars, visited, depth+1))
		l2 := strings.Fields(e.expandInternal(args[1], autoVars, visited, depth+1))
		maxLen := len(l1)
		if len(l2) > maxLen {
			maxLen = len(l2)
		}
		var results []string
		for i := 0; i < maxLen; i++ {
			p1, p2 := "", ""
			if i < len(l1) {
				p1 = l1[i]
			}
			if i < len(l2) {
				p2 = l2[i]
			}
			results = append(results, p1+p2)
		}
		return strings.Join(results, " ")

	case "if":
		args := splitFuncArgs(argsRaw)
		if len(args) < 2 {
			return ""
		}
		cond := strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1))
		if cond != "" {
			return strings.TrimSpace(e.expandInternal(args[1], autoVars, visited, depth+1))
		}
		if len(args) >= 3 {
			return strings.TrimSpace(e.expandInternal(args[2], autoVars, visited, depth+1))
		}
		return ""

	case "or":
		args := splitFuncArgs(argsRaw)
		for _, arg := range args {
			v := strings.TrimSpace(e.expandInternal(arg, autoVars, visited, depth+1))
			if v != "" {
				return v
			}
		}
		return ""

	case "and":
		args := splitFuncArgs(argsRaw)
		if len(args) == 0 {
			return ""
		}
		last := ""
		for _, arg := range args {
			last = strings.TrimSpace(e.expandInternal(arg, autoVars, visited, depth+1))
			if last == "" {
				return ""
			}
		}
		return last

	case "foreach":
		args := splitFuncArgs(argsRaw)
		if len(args) < 3 {
			return ""
		}
		varName := strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1))
		list := strings.Fields(e.expandInternal(args[1], autoVars, visited, depth+1))
		template := strings.TrimSpace(args[2])

		childScope := NewChildVarTable(e.Scope)
		childExpander := &Expander{Scope: childScope, MaxDepth: e.MaxDepth, ShellFn: e.ShellFn, WarnFn: e.WarnFn}
		var results []string
		for idx, item := range list {
			if idx >= 1000 {
				break
			}
			childScope.Set(varName, item, OpSimple, nil, 0)
			res := childExpander.expandInternal(template, autoVars, visited, depth+1)
			results = append(results, res)
		}
		return strings.Join(results, " ")

	case "call":
		args := splitFuncArgs(argsRaw)
		if len(args) == 0 {
			return ""
		}
		varName := strings.TrimSpace(e.expandInternal(args[0], autoVars, visited, depth+1))
		v, ok := e.Scope.Get(varName)
		if !ok {
			return ""
		}
		childScope := NewChildVarTable(e.Scope)
		childScope.Set("0", varName, OpSimple, nil, 0)
		for idx := 1; idx < len(args); idx++ {
			paramVal := strings.TrimSpace(e.expandInternal(args[idx], autoVars, visited, depth+1))
			childScope.Set(strconv.Itoa(idx), paramVal, OpSimple, nil, 0)
		}
		childExpander := &Expander{Scope: childScope, MaxDepth: e.MaxDepth, ShellFn: e.ShellFn, WarnFn: e.WarnFn}
		return childExpander.expandInternal(v.RawValue, autoVars, visited, depth+1)

	case "shell":
		cmd := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		return executeShellCommand(cmd, e)

	case "eval":
		e.expandInternal(argsRaw, autoVars, visited, depth+1)
		return ""

	case "info", "warning", "error":
		msg := e.expandInternal(argsRaw, autoVars, visited, depth+1)
		if e.WarnFn != nil {
			e.WarnFn("[make:%s] %s", funcName, msg)
		}
		return ""

	case "origin":
		varName := strings.TrimSpace(e.expandInternal(argsRaw, autoVars, visited, depth+1))
		if v, ok := e.Scope.Get(varName); ok {
			return v.Origin
		}
		if _, ok := os.LookupEnv(varName); ok {
			return "environment"
		}
		return "undefined"

	case "flavor":
		varName := strings.TrimSpace(e.expandInternal(argsRaw, autoVars, visited, depth+1))
		if v, ok := e.Scope.Get(varName); ok {
			if v.Flavor == VarFlavorSimple {
				return "simple"
			}
			return "recursive"
		}
		return "undefined"

	default:
		if e.WarnFn != nil {
			e.WarnFn("ignoring unknown make function '$(%s)'", funcName)
		}
		return ""
	}
}

func matchPattern(pattern, text string) bool {
	prefix, suffix, hasPercent := splitPattern(pattern)
	if !hasPercent {
		return pattern == text
	}
	return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix) &&
		len(text) >= len(prefix)+len(suffix)
}

func executeShellCommand(cmd string, e *Expander) string {
	if e != nil && e.ShellFn != nil {
		out, err := e.ShellFn(cmd)
		if err != nil && e.WarnFn != nil {
			e.WarnFn("shell command '%s' failed: %v", cmd, err)
		}
		return sanitizeShellOutput(out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmdExec := exec.CommandContext(ctx, "sh", "-c", cmd)
	out, err := cmdExec.Output()
	if err != nil && e != nil && e.WarnFn != nil {
		e.WarnFn("shell command execution failed: %v", err)
	}
	return sanitizeShellOutput(string(out))
}

func sanitizeShellOutput(out string) string {
	out = strings.ReplaceAll(out, "\r\n", " ")
	out = strings.ReplaceAll(out, "\n", " ")
	return strings.TrimRight(out, " \t")
}

// CondFrame tracks conditional branch state.
type CondFrame struct {
	ConditionMet bool // True if any branch in this if-chain has been satisfied
	Active       bool // True if current branch is actively parsed
	ParentActive bool // True if parent conditional frame is active
}

// CondStack manages nested conditional directives.
type CondStack struct {
	frames []CondFrame
}

// IsActive returns whether code lines should be actively parsed at the current condition depth.
func (s *CondStack) IsActive() bool {
	if len(s.frames) == 0 {
		return true
	}
	return s.frames[len(s.frames)-1].Active
}

// parseIfArgs parses conditional arguments for ifeq/ifneq: (arg1, arg2) or 'arg1' 'arg2' or "arg1" "arg2".
func parseIfArgs(args string, expander *Expander) (string, string, error) {
	trimmed := strings.TrimSpace(args)
	if strings.HasPrefix(trimmed, "(") {
		endParen := strings.LastIndexByte(trimmed, ')')
		if endParen == -1 {
			return "", "", fmt.Errorf("missing closing parenthesis in conditional: %s", args)
		}
		inner := trimmed[1:endParen]
		commaIdx := findTopLevelComma(inner)
		if commaIdx == -1 {
			return "", "", fmt.Errorf("missing comma in conditional: %s", args)
		}
		arg1 := strings.TrimSpace(inner[:commaIdx])
		arg2 := strings.TrimSpace(inner[commaIdx+1:])
		if expander != nil {
			arg1 = expander.Expand(arg1, nil)
			arg2 = expander.Expand(arg2, nil)
		}
		return strings.TrimSpace(arg1), strings.TrimSpace(arg2), nil
	}

	// Quoted arguments: 'arg1' 'arg2' or "arg1" "arg2"
	var tokens []string
	i := 0
	for i < len(trimmed) {
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
			i++
		}
		if i >= len(trimmed) {
			break
		}
		quote := trimmed[i]
		if quote == '\'' || quote == '"' {
			i++
			start := i
			for i < len(trimmed) && trimmed[i] != quote {
				i++
			}
			tokens = append(tokens, trimmed[start:i])
			if i < len(trimmed) {
				i++
			}
		} else {
			start := i
			for i < len(trimmed) && trimmed[i] != ' ' && trimmed[i] != '\t' {
				i++
			}
			tokens = append(tokens, trimmed[start:i])
		}
	}

	if len(tokens) >= 2 {
		arg1 := tokens[0]
		arg2 := tokens[1]
		if expander != nil {
			arg1 = expander.Expand(arg1, nil)
			arg2 = expander.Expand(arg2, nil)
		}
		return strings.TrimSpace(arg1), strings.TrimSpace(arg2), nil
	}

	return "", "", fmt.Errorf("cannot parse conditional arguments: %s", args)
}

func findTopLevelComma(s string) int {
	depthParen := 0
	depthBrace := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case ',':
			if depthParen == 0 && depthBrace == 0 {
				return i
			}
		}
	}
	return -1
}

// evaluateCondition evaluates ifeq, ifneq, ifdef, ifndef.
func evaluateCondition(condType DirectiveType, args string, expander *Expander, scope *VarTable) bool {
	switch condType {
	case DirectiveIfEq:
		arg1, arg2, err := parseIfArgs(args, expander)
		if err != nil {
			return false
		}
		return arg1 == arg2

	case DirectiveIfNeq:
		arg1, arg2, err := parseIfArgs(args, expander)
		if err != nil {
			return false
		}
		return arg1 != arg2

	case DirectiveIfDef:
		varName := strings.TrimSpace(args)
		if strings.HasPrefix(varName, "$(") && strings.HasSuffix(varName, ")") && expander != nil {
			varName = expander.Expand(varName, nil)
		} else if strings.HasPrefix(varName, "${") && strings.HasSuffix(varName, "}") && expander != nil {
			varName = expander.Expand(varName, nil)
		}
		varName = strings.TrimSpace(varName)
		if scope != nil {
			if v, ok := scope.Get(varName); ok {
				return strings.TrimSpace(v.RawValue) != "" || strings.TrimSpace(v.Value) != ""
			}
		}
		if val, ok := os.LookupEnv(varName); ok {
			return strings.TrimSpace(val) != ""
		}
		return false

	case DirectiveIfNDef:
		return !evaluateCondition(DirectiveIfDef, args, expander, scope)
	}

	return false
}

// handleConditionalDirective processes conditional directives and returns true if handled.
func handleConditionalDirective(dir *Directive, stack *CondStack, expander *Expander, scope *VarTable, ast *MakefileAST) bool {
	switch dir.Type {
	case DirectiveIfEq, DirectiveIfNeq, DirectiveIfDef, DirectiveIfNDef:
		parentActive := stack.IsActive()
		condResult := false
		if parentActive {
			condResult = evaluateCondition(dir.Type, dir.Args, expander, scope)
		}
		stack.frames = append(stack.frames, CondFrame{
			ConditionMet: condResult,
			Active:       parentActive && condResult,
			ParentActive: parentActive,
		})
		return true

	case DirectiveElse:
		if len(stack.frames) == 0 {
			ast.AddDiagnostic(dir.LineNumber, "unexpected 'else' without matching 'if'", SeverityWarn)
			return true
		}
		top := &stack.frames[len(stack.frames)-1]
		if !top.ParentActive {
			top.Active = false
			return true
		}

		argsTrimmed := strings.TrimSpace(dir.Args)
		if strings.HasPrefix(argsTrimmed, "ifeq") || strings.HasPrefix(argsTrimmed, "ifneq") ||
			strings.HasPrefix(argsTrimmed, "ifdef") || strings.HasPrefix(argsTrimmed, "ifndef") {
			fields := strings.Fields(argsTrimmed)
			subType := DirectiveType(fields[0])
			subArgs := strings.TrimSpace(strings.TrimPrefix(argsTrimmed, fields[0]))

			if top.ConditionMet {
				top.Active = false
			} else {
				subResult := evaluateCondition(subType, subArgs, expander, scope)
				if subResult {
					top.ConditionMet = true
					top.Active = true
				} else {
					top.Active = false
				}
			}
		} else {
			// Plain else
			if top.ConditionMet {
				top.Active = false
			} else {
				top.ConditionMet = true
				top.Active = true
			}
		}
		return true

	case DirectiveEndIf:
		if len(stack.frames) == 0 {
			ast.AddDiagnostic(dir.LineNumber, "unexpected 'endif' without matching 'if'", SeverityWarn)
			return true
		}
		stack.frames = stack.frames[:len(stack.frames)-1]
		return true
	}

	return false
}

// parseDefineBlock extracts multi-line variable definitions.
func parseDefineBlock(lines []*LogicalLine, startIdx int, expander *Expander, scope *VarTable, ast *MakefileAST) (int, bool) {
	line := lines[startIdx]
	trimmed := strings.TrimSpace(line.Text)
	if !strings.HasPrefix(trimmed, "define ") && trimmed != "define" {
		return startIdx, false
	}

	header := strings.TrimSpace(strings.TrimPrefix(trimmed, "define"))
	op := OpRecursive
	varName := header

	if opIdx := FindAssignmentOperator(header); opIdx != -1 {
		varName = strings.TrimSpace(header[:opIdx])
		opPart := strings.TrimSpace(header[opIdx:])
		if strings.HasPrefix(opPart, ":=") {
			op = OpSimple
		} else if strings.HasPrefix(opPart, "::=") {
			op = OpPOSIX
		} else if strings.HasPrefix(opPart, "?=") {
			op = OpDefault
		} else if strings.HasPrefix(opPart, "+=") {
			op = OpAppend
		} else if strings.HasPrefix(opPart, "!=") {
			op = OpShell
		}
	}

	var body strings.Builder
	currIdx := startIdx + 1

	for currIdx < len(lines) {
		bodyLine := lines[currIdx]
		bodyTrimmed := strings.TrimSpace(bodyLine.Raw)
		if bodyTrimmed == "endef" || strings.HasPrefix(bodyTrimmed, "endef ") || strings.HasPrefix(bodyTrimmed, "endef\t") {
			break
		}
		if body.Len() > 0 {
			body.WriteString("\n")
		}
		body.WriteString(bodyLine.Raw)
		currIdx++
	}

	rawBody := body.String()
	scope.Set(varName, rawBody, op, expander, line.StartLine)

	evalVal := ""
	if op == OpSimple || op == OpPOSIX {
		evalVal = expander.Expand(rawBody, nil)
	} else {
		evalVal = rawBody
	}

	ast.AddVariable(VariableAssignment{
		Name:       varName,
		Value:      evalVal,
		RawValue:   rawBody,
		Flavor:     AssignmentFlavor(op),
		LineNumber: line.StartLine,
	})

	return currIdx, true
}

// parseStaticPatternRule resolves static pattern rules: targets ... : target-pattern : prereq-patterns ...
func parseStaticPatternRule(header string, expander *Expander, lineNum int) (*MakefileRule, bool) {
	// Must have 2 colons outside parentheses/braces/quotes
	clean := strings.TrimSpace(StripComments(header))
	var colons []int
	inParen := 0
	inBrace := 0
	inQuote := byte(0)

	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if inQuote != 0 {
			if c == inQuote && (i == 0 || clean[i-1] != '\\') {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '(' {
			inParen++
			continue
		}
		if c == ')' && inParen > 0 {
			inParen--
			continue
		}
		if c == '{' {
			inBrace++
			continue
		}
		if c == '}' && inBrace > 0 {
			inBrace--
			continue
		}
		if inParen == 0 && inBrace == 0 && c == ':' {
			colons = append(colons, i)
		}
	}

	if len(colons) != 2 {
		return nil, false
	}
	if colons[1] == colons[0]+1 {
		return nil, false // Double colon ::
	}

	targetPart := strings.TrimSpace(clean[:colons[0]])
	targetPattern := strings.TrimSpace(clean[colons[0]+1 : colons[1]])
	afterSecondColon := clean[colons[1]+1:]

	prereqPart := afterSecondColon
	semiIdx := FindSemicolon(afterSecondColon)
	inlineRecipe := ""
	if semiIdx != -1 {
		prereqPart = afterSecondColon[:semiIdx]
		inlineRecipe = strings.TrimSpace(afterSecondColon[semiIdx+1:])
	}

	if expander != nil {
		targetPart = expander.Expand(targetPart, nil)
		targetPattern = expander.Expand(targetPattern, nil)
		prereqPart = expander.Expand(prereqPart, nil)
	}

	targets := strings.Fields(targetPart)
	prereqPatterns := strings.Fields(prereqPart)

	var resolvedPrereqs []string
	targetPrefix, targetSuffix, targetHasPct := splitPattern(targetPattern)

	for _, t := range targets {
		stem := t
		if targetHasPct && strings.HasPrefix(t, targetPrefix) && strings.HasSuffix(t, targetSuffix) && len(t) >= len(targetPrefix)+len(targetSuffix) {
			stem = t[len(targetPrefix) : len(t)-len(targetSuffix)]
		}
		for _, pp := range prereqPatterns {
			pPrefix, pSuffix, pHasPct := splitPattern(pp)
			if pHasPct {
				resolvedPrereqs = append(resolvedPrereqs, pPrefix+stem+pSuffix)
			} else {
				resolvedPrereqs = append(resolvedPrereqs, pp)
			}
		}
	}

	rule := &MakefileRule{
		Targets:       targets,
		Prerequisites: resolvedPrereqs,
		Recipes:       make([]string, 0),
		RecipeLines:   make([]RecipeLine, 0),
		IsPattern:     false,
		IsDoubleColon: false,
		LineNumber:    lineNum,
	}

	if inlineRecipe != "" {
		rec := ParseRecipeLine(inlineRecipe, lineNum)
		rule.RecipeLines = append(rule.RecipeLines, rec)
		rule.Recipes = append(rule.Recipes, rec.Command)
	}

	return rule, true
}

// ParseMakefile parses Makefile content into an intermediate MakefileAST.
func ParseMakefile(content string) (*MakefileAST, error) {
	ast := NewMakefileAST()
	scope := NewVarTable()
	expander := NewExpander(scope)
	expander.WarnFn = func(format string, args ...any) {
		ast.AddDiagnostic(0, fmt.Sprintf(format, args...), SeverityWarn)
	}

	ast.Variables = scope.ToMap(expander)
	ast.RawVars = scope.ToRawMap()

	if strings.TrimSpace(content) == "" {
		return ast, nil
	}

	lines := NormalizeLines(content)

	// Step 1: Pass 1 - Conditionals, Variables, Includes, Directives
	condStack := &CondStack{}
	var activeLines []*LogicalLine

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// 1. Check multi-line define block
		if condStack.IsActive() && (strings.HasPrefix(strings.TrimSpace(line.Text), "define ") || strings.TrimSpace(line.Text) == "define") {
			nextIdx, handled := parseDefineBlock(lines, i, expander, scope, ast)
			if handled {
				i = nextIdx
				continue
			}
		}

		// 2. Check directive
		clean := strings.TrimSpace(StripComments(line.Text))
		if LooksLikeDirective(clean) {
			dir, err := ParseDirective(clean, line.StartLine)
			if err == nil {
				if handleConditionalDirective(dir, condStack, expander, scope, ast) {
					continue
				}

				if !condStack.IsActive() {
					continue
				}

				ast.Directives = append(ast.Directives, dir)

				// Handle include / -include / sinclude
				if dir.Type == DirectiveInclude || dir.Type == DirectiveOptionalInclude || dir.Type == DirectiveSInclude {
					expandedArgs := expander.Expand(dir.Args, nil)
					incFiles := strings.Fields(expandedArgs)
					ast.IncludedFiles = append(ast.IncludedFiles, incFiles...)

					for _, f := range incFiles {
						incData, readErr := os.ReadFile(f)
						if readErr != nil {
							if dir.Type == DirectiveInclude {
								ast.AddDiagnostic(line.StartLine, fmt.Sprintf("include file '%s' not found: %v", f, readErr), SeverityWarn)
							}
						} else {
							// Recursively parse included file into current scope & AST
							incAST, incErr := ParseMakefile(string(incData))
							if incErr == nil && incAST != nil {
								for k, v := range incAST.Variables {
									scope.Set(k, v, OpSimple, expander, line.StartLine)
									ast.AddVariable(VariableAssignment{Name: k, Value: v, RawValue: incAST.RawVars[k], Flavor: FlavorSimple})
								}
								for _, r := range incAST.Rules {
									ast.AddRule(r)
								}
								for t := range incAST.PhonyTargets {
									ast.PhonyTargets[t] = true
								}
							}
						}
					}
					continue
				}

				// Handle export / unexport / override
				if dir.Type == DirectiveExport || dir.Type == DirectiveOverride {
					if LooksLikeVariableAssignment(dir.Args) {
						assign, aErr := ParseVariableAssignment(dir.Args, line.StartLine)
						if aErr == nil {
							scope.Set(assign.Name, assign.Value, VarAssignOp(assign.Flavor), expander, line.StartLine)
							assign.Value = expander.Expand(assign.Value, nil)
							ast.AddVariable(*assign)
							continue
						}
					}
				}

				continue
			}
		}

		if !condStack.IsActive() {
			continue
		}

		// 3. Check variable assignment outside rule context (non-tab lines)
		if !line.LeadingTab && LooksLikeVariableAssignment(clean) {
			assign, err := ParseVariableAssignment(clean, line.StartLine)
			if err == nil {
				scope.Set(assign.Name, assign.Value, VarAssignOp(assign.Flavor), expander, line.StartLine)
				evalVal := ""
				if assign.Flavor == FlavorSimple || assign.Flavor == FlavorSimplePOSIX {
					evalVal = expander.Expand(assign.Value, nil)
				} else {
					evalVal = assign.Value
				}
				assign.Value = evalVal
				ast.AddVariable(*assign)
				continue
			}
		}

		activeLines = append(activeLines, line)
	}

	if len(condStack.frames) > 0 {
		ast.AddDiagnostic(0, "unclosed conditional directive, expected 'endif'", SeverityWarn)
	}

	// Update AST Variables dictionary from scope table
	ast.Variables = scope.ToMap(expander)
	ast.RawVars = scope.ToRawMap()

	// Step 2: Pass 2 - Target Rules and Recipes
	var currentRule *MakefileRule
	inRule := false

	for _, line := range activeLines {
		rawTrimmed := strings.TrimSpace(line.Raw)
		if rawTrimmed == "" {
			continue
		}

		// Recipe line check (leading tab or indented in rule context)
		if (line.LeadingTab || (inRule && line.LeadingSpaces >= 2 && !LooksLikeRuleHeader(line.Text) && !LooksLikeDirective(line.Text))) && currentRule != nil {
			rec := ParseRecipeLine(line.Raw, line.StartLine)
			currentRule.RecipeLines = append(currentRule.RecipeLines, rec)
			currentRule.Recipes = append(currentRule.Recipes, rec.Command)
			continue
		}

		// Non-recipe line terminates current rule recipe collection
		inRule = false
		currentRule = nil

		clean := strings.TrimSpace(StripComments(line.Text))
		if clean == "" {
			continue
		}

		// Check for static pattern rule: targets: pattern: prereqs
		if staticRule, isStatic := parseStaticPatternRule(clean, expander, line.StartLine); isStatic {
			ast.AddRule(staticRule)
			currentRule = staticRule
			inRule = true
			continue
		}

		// Check for target rule header
		if LooksLikeRuleHeader(clean) {
			targets, prereqs, inlineRecipe, isDoubleColon, isPattern, err := ParseRuleHeader(clean, line.StartLine)
			if err != nil {
				ast.AddDiagnostic(line.StartLine, fmt.Sprintf("failed to parse rule header: %v", err), SeverityWarn)
				continue
			}

			// Expand targets and prerequisites
			var expandedTargets []string
			for _, t := range targets {
				exp := expander.Expand(t, nil)
				expandedTargets = append(expandedTargets, strings.Fields(exp)...)
			}

			// Handle order-only prerequisites (|)
			var normalPrereqs []string
			var orderOnlyPrereqs []string
			inOrderOnly := false

			for _, p := range prereqs {
				if p == "|" {
					inOrderOnly = true
					continue
				}
				if strings.HasPrefix(p, "|") {
					inOrderOnly = true
					p = strings.TrimPrefix(p, "|")
					if p == "" {
						continue
					}
				}
				exp := expander.Expand(p, nil)
				pFields := strings.Fields(exp)
				if inOrderOnly {
					orderOnlyPrereqs = append(orderOnlyPrereqs, pFields...)
				} else {
					normalPrereqs = append(normalPrereqs, pFields...)
				}
			}

			// Check if this is .PHONY declaration
			isPhonyDecl := false
			for _, t := range expandedTargets {
				if t == ".PHONY" {
					isPhonyDecl = true
					break
				}
			}

			if isPhonyDecl {
				ast.MarkPhony(normalPrereqs...)
				continue
			}

			// Check pattern status
			for _, t := range expandedTargets {
				if strings.Contains(t, "%") {
					isPattern = true
					break
				}
			}

			rule := &MakefileRule{
				Targets:       expandedTargets,
				Prerequisites: normalPrereqs,
				OrderOnly:     orderOnlyPrereqs,
				Recipes:       make([]string, 0),
				RecipeLines:   make([]RecipeLine, 0),
				IsPhony:       false,
				IsPattern:     isPattern,
				IsDoubleColon: isDoubleColon,
				LineNumber:    line.StartLine,
			}

			// Check if any target is already known to be phony
			for _, t := range expandedTargets {
				if ast.PhonyTargets[t] {
					rule.IsPhony = true
					break
				}
			}

			if inlineRecipe != "" {
				rec := ParseRecipeLine(inlineRecipe, line.StartLine)
				rule.RecipeLines = append(rule.RecipeLines, rec)
				rule.Recipes = append(rule.Recipes, rec.Command)
			}

			ast.AddRule(rule)
			currentRule = rule
			inRule = true
			continue
		}

		// Malformed or unrecognized line
		ast.AddDiagnostic(line.StartLine, fmt.Sprintf("unrecognized line: %s", clean), SeverityWarn)
	}

	// Post-processing: Synchronize phony targets across all rules
	for _, rule := range ast.Rules {
		for _, t := range rule.Targets {
			if ast.PhonyTargets[t] {
				rule.IsPhony = true
				break
			}
		}
	}

	return ast, nil
}
