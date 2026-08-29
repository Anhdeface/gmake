package converter

import (
	"bufio"
	"fmt"
	"strings"
	"unicode"
)

// LineType represents the classification of a normalized logical line.
type LineType int

const (
	LineTypeBlank      LineType = iota // Empty line, whitespace-only, or full-line comment
	LineTypeVariable                   // Variable assignment (VAR = val, VAR := val, etc.)
	LineTypeRule                       // Target rule header (target: prereqs)
	LineTypeRecipe                     // Recipe command line (leading TAB or indented)
	LineTypeDirective                  // Makefile directive (include, ifeq, export, etc.)
	LineTypeMalformed                  // Unrecognized or malformed non-blank line
)

// String returns the string representation of LineType.
func (t LineType) String() string {
	switch t {
	case LineTypeBlank:
		return "Blank"
	case LineTypeVariable:
		return "Variable"
	case LineTypeRule:
		return "Rule"
	case LineTypeRecipe:
		return "Recipe"
	case LineTypeDirective:
		return "Directive"
	case LineTypeMalformed:
		return "Malformed"
	default:
		return "Unknown"
	}
}

// LogicalLine represents a normalized line spanning one or more physical lines.
type LogicalLine struct {
	Raw           string   // Original concatenated raw lines (including continuations)
	RawLine       string   // Alias for Raw (original first raw line / combined raw text)
	Text          string   // Normalized line text (continuations joined)
	Content       string   // Alias for Text
	StartLine     int      // 1-based starting physical line number
	EndLine       int      // 1-based ending physical line number
	LineNumber    int      // Alias for StartLine
	LeadingTab    bool     // True if physical line started with a Tab character
	IsTabIndented bool     // Alias for LeadingTab
	LeadingSpaces int      // Number of leading spaces if not starting with Tab
	Type          LineType // Classified line type
}

// CountTrailingBackslashes counts consecutive trailing backslashes at end of string.
func CountTrailingBackslashes(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\\' {
			count++
		} else {
			break
		}
	}
	return count
}

// StripComments removes Makefile comments starting with '#' outside single/double quotes.
func StripComments(s string) string {
	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			sb.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			sb.WriteByte(ch)
			continue
		}

		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			sb.WriteByte(ch)
			continue
		}

		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			sb.WriteByte(ch)
			continue
		}

		if ch == '#' && !inSingleQuote && !inDoubleQuote {
			break
		}

		sb.WriteByte(ch)
	}

	return sb.String()
}

// NormalizeLines pre-processes Makefile content:
// 1. Normalizes CRLF and CR newlines to LF.
// 2. Merges multi-line continuations ending with odd trailing backslashes '\'.
// 3. Tracks 1-based start and end physical line numbers for each logical line.
func NormalizeLines(content string) []*LogicalLine {
	var logicalLines []*LogicalLine

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentRaw strings.Builder
	var currentText strings.Builder
	startLine := 0
	currentPhysicalLine := 0
	isContinuation := false
	leadingTab := false
	leadingSpaces := 0

	for scanner.Scan() {
		currentPhysicalLine++
		line := scanner.Text()

		if !isContinuation {
			startLine = currentPhysicalLine
			leadingTab = strings.HasPrefix(line, "\t")
			leadingSpaces = 0
			if !leadingTab {
				for _, r := range line {
					if r == ' ' {
						leadingSpaces++
					} else {
						break
					}
				}
			}
		}

		// Check for line continuation with backslash
		trimmedRight := strings.TrimRight(line, " \t")
		numTrailingBackslashes := CountTrailingBackslashes(trimmedRight)
		hasContinuation := (numTrailingBackslashes%2 == 1)

		lineSegment := line
		if hasContinuation {
			lineSegment = trimmedRight[:len(trimmedRight)-1]
		}

		if isContinuation {
			currentRaw.WriteString("\n")
			currentRaw.WriteString(line)

			trimmedLeft := strings.TrimLeft(lineSegment, " \t")
			if currentText.Len() > 0 && !strings.HasSuffix(currentText.String(), " ") && trimmedLeft != "" {
				currentText.WriteString(" ")
			}
			currentText.WriteString(trimmedLeft)
		} else {
			currentRaw.WriteString(line)
			currentText.WriteString(lineSegment)
		}

		if hasContinuation {
			isContinuation = true
		} else {
			rawStr := currentRaw.String()
			textStr := currentText.String()
			logicalLines = append(logicalLines, &LogicalLine{
				Raw:           rawStr,
				RawLine:       rawStr,
				Text:          textStr,
				Content:       textStr,
				StartLine:     startLine,
				EndLine:       currentPhysicalLine,
				LineNumber:    startLine,
				LeadingTab:    leadingTab,
				IsTabIndented: leadingTab,
				LeadingSpaces: leadingSpaces,
			})
			currentRaw.Reset()
			currentText.Reset()
			isContinuation = false
		}
	}

	if isContinuation && currentRaw.Len() > 0 {
		rawStr := currentRaw.String()
		textStr := currentText.String()
		logicalLines = append(logicalLines, &LogicalLine{
			Raw:           rawStr,
			RawLine:       rawStr,
			Text:          textStr,
			Content:       textStr,
			StartLine:     startLine,
			EndLine:       currentPhysicalLine,
			LineNumber:    startLine,
			LeadingTab:    leadingTab,
			IsTabIndented: leadingTab,
			LeadingSpaces: leadingSpaces,
		})
	}

	return logicalLines
}

// NormalizeMakefileLines returns normalized logical lines and any warnings encountered.
func NormalizeMakefileLines(content string) ([]*LogicalLine, []string) {
	lines := NormalizeLines(content)
	var warnings []string
	return lines, warnings
}

// ClassifyLine categorizes a LogicalLine into its respective LineType.
func ClassifyLine(line *LogicalLine, inRuleContext bool) LineType {
	rawTrimmed := strings.TrimSpace(line.Raw)
	if rawTrimmed == "" {
		return LineTypeBlank
	}

	// Full-line comments
	if strings.HasPrefix(rawTrimmed, "#") {
		if line.LeadingTab && inRuleContext {
			return LineTypeRecipe
		}
		return LineTypeBlank
	}

	// Explicit recipe line (indented with Tab)
	if line.LeadingTab || line.IsTabIndented {
		return LineTypeRecipe
	}

	// Recipe line indented with 4+ spaces in rule context
	if inRuleContext && line.LeadingSpaces >= 4 {
		if !LooksLikeRuleHeader(line.Text) && !LooksLikeVariableAssignment(line.Text) && !LooksLikeDirective(line.Text) {
			return LineTypeRecipe
		}
	}

	clean := strings.TrimSpace(StripComments(line.Text))
	if clean == "" {
		return LineTypeBlank
	}

	if LooksLikeDirective(clean) {
		return LineTypeDirective
	}

	if LooksLikeVariableAssignment(clean) {
		return LineTypeVariable
	}

	if LooksLikeRuleHeader(clean) {
		return LineTypeRule
	}

	return LineTypeMalformed
}

// LooksLikeDirective checks if a string begins with a known Makefile directive keyword.
func LooksLikeDirective(s string) bool {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if idx := strings.Index(first, "("); idx != -1 {
		first = first[:idx]
	}

	switch first {
	case "include", "-include", "sinclude", "ifeq", "ifneq", "ifdef", "ifndef", "else", "endif", "vpath":
		return true
	case "export", "override", "unexport":
		if (first == "export" || first == "override") && len(fields) > 1 {
			if LooksLikeVariableAssignment(strings.TrimPrefix(s, first)) {
				return false // Allow variable assignment parser to capture "export VAR = val"
			}
		}
		return true
	}
	return false
}

// LooksLikeVariableAssignment checks if a string is a variable assignment construct.
func LooksLikeVariableAssignment(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "export ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "export"))
	} else if strings.HasPrefix(s, "override ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "override"))
	}

	opIdx := FindAssignmentOperator(s)
	if opIdx == -1 {
		return false
	}

	namePart := strings.TrimSpace(s[:opIdx])
	if namePart == "" {
		return false
	}

	if strings.Contains(namePart, ":") {
		return false
	}

	if strings.ContainsAny(namePart, " \t\r\n") {
		return false
	}

	return true
}

// FindAssignmentOperator locates the index of an assignment operator (=, :=, ::=, ?=, +=, !=)
// outside quotes, parentheses, and brace expansions.
func FindAssignmentOperator(s string) int {
	inParen := 0
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if inDoubleQuote || inSingleQuote {
			continue
		}

		if ch == '(' || ch == '{' {
			inParen++
			continue
		}
		if ch == ')' || ch == '}' {
			if inParen > 0 {
				inParen--
			}
			continue
		}
		if inParen > 0 {
			continue
		}

		if i+2 < len(s) {
			three := s[i : i+3]
			if three == "::=" {
				return i
			}
		}
		if i+1 < len(s) {
			two := s[i : i+2]
			if two == ":=" || two == "?=" || two == "+=" || two == "!=" {
				return i
			}
		}
		if ch == '=' {
			return i
		}
	}

	return -1
}

// LooksLikeRuleHeader determines if a string is a target rule header (contains ':' outside quotes & macros).
func LooksLikeRuleHeader(s string) bool {
	inParen := 0
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if inDoubleQuote || inSingleQuote {
			continue
		}

		if ch == '(' || ch == '{' {
			inParen++
			continue
		}
		if ch == ')' || ch == '}' {
			if inParen > 0 {
				inParen--
			}
			continue
		}
		if inParen > 0 {
			continue
		}

		if ch == ':' {
			if i+1 < len(s) && s[i+1] == '=' {
				return false
			}
			if i+2 < len(s) && s[i:i+3] == "::=" {
				return false
			}
			return true
		}
	}

	return false
}

// ParseVariableAssignment parses a variable assignment line into a VariableAssignment struct.
func ParseVariableAssignment(s string, lineNum int) (*VariableAssignment, error) {
	clean := strings.TrimSpace(StripComments(s))
	exported := false
	override := false

	if strings.HasPrefix(clean, "export ") {
		exported = true
		clean = strings.TrimSpace(strings.TrimPrefix(clean, "export"))
	} else if strings.HasPrefix(clean, "override ") {
		override = true
		clean = strings.TrimSpace(strings.TrimPrefix(clean, "override"))
	}

	opIdx := FindAssignmentOperator(clean)
	if opIdx == -1 {
		return nil, fmt.Errorf("no assignment operator found in line: %s", s)
	}

	name := strings.TrimSpace(clean[:opIdx])
	rest := clean[opIdx:]

	var flavor AssignmentFlavor
	var val string

	if strings.HasPrefix(rest, "::=") {
		flavor = FlavorSimplePOSIX
		val = strings.TrimSpace(rest[3:])
	} else if strings.HasPrefix(rest, ":=") {
		flavor = FlavorSimple
		val = strings.TrimSpace(rest[2:])
	} else if strings.HasPrefix(rest, "?=") {
		flavor = FlavorConditional
		val = strings.TrimSpace(rest[2:])
	} else if strings.HasPrefix(rest, "+=") {
		flavor = FlavorAppend
		val = strings.TrimSpace(rest[2:])
	} else if strings.HasPrefix(rest, "!=") {
		flavor = FlavorShell
		val = strings.TrimSpace(rest[2:])
	} else if strings.HasPrefix(rest, "=") {
		flavor = FlavorRecursive
		val = strings.TrimSpace(rest[1:])
	} else {
		return nil, fmt.Errorf("unknown assignment flavor in: %s", s)
	}

	return &VariableAssignment{
		Name:       name,
		Value:      val,
		RawValue:   val,
		Flavor:     flavor,
		Exported:   exported,
		Override:   override,
		LineNumber: lineNum,
	}, nil
}

// ParseRuleHeader parses a target rule header string into targets, prerequisites, inline recipe,
// double-colon flag, and pattern rule flag.
func ParseRuleHeader(s string, lineNum int) (targets []string, prereqs []string, inlineRecipe string, isDoubleColon bool, isPattern bool, err error) {
	clean := strings.TrimSpace(StripComments(s))

	inParen := 0
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false
	colonIdx := -1

	for i := 0; i < len(clean); i++ {
		ch := clean[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if inDoubleQuote || inSingleQuote {
			continue
		}
		if ch == '(' || ch == '{' {
			inParen++
			continue
		}
		if ch == ')' || ch == '}' {
			if inParen > 0 {
				inParen--
			}
			continue
		}
		if inParen > 0 {
			continue
		}

		if ch == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx == -1 {
		return nil, nil, "", false, false, fmt.Errorf("missing colon in rule header: %s", s)
	}

	targetPart := strings.TrimSpace(clean[:colonIdx])
	afterColon := clean[colonIdx+1:]

	if strings.HasPrefix(afterColon, ":") {
		isDoubleColon = true
		afterColon = afterColon[1:]
	}

	prereqPart := afterColon
	inlineRecipe = ""

	semiIdx := FindSemicolon(afterColon)
	if semiIdx != -1 {
		prereqPart = afterColon[:semiIdx]
		inlineRecipe = strings.TrimSpace(afterColon[semiIdx+1:])
	}

	targets = SplitTokens(targetPart)
	prereqs = SplitTokens(prereqPart)

	for _, t := range targets {
		if strings.Contains(t, "%") {
			isPattern = true
			break
		}
	}

	return targets, prereqs, inlineRecipe, isDoubleColon, isPattern, nil
}

// FindSemicolon locates the index of ';' outside quotes, parentheses, and brace expansions.
func FindSemicolon(s string) int {
	inParen := 0
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if inDoubleQuote || inSingleQuote {
			continue
		}
		if ch == '(' || ch == '{' {
			inParen++
			continue
		}
		if ch == ')' || ch == '}' {
			if inParen > 0 {
				inParen--
			}
			continue
		}
		if inParen > 0 {
			continue
		}
		if ch == ';' {
			return i
		}
	}
	return -1
}

// ParseDirective extracts the directive keyword and unparsed arguments into a Directive struct.
func ParseDirective(line string, lineNum int) (*Directive, error) {
	trimmed := strings.TrimSpace(StripComments(line))
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty line cannot be directive")
	}

	first := fields[0]
	args := ""
	if idx := strings.Index(first, "("); idx != -1 {
		prefix := first[:idx]
		args = strings.TrimSpace(trimmed[len(prefix):])
		first = prefix
	} else {
		args = strings.TrimSpace(trimmed[len(first):])
	}

	var dType DirectiveType
	switch first {
	case "include":
		dType = DirectiveInclude
	case "-include":
		dType = DirectiveOptionalInclude
	case "sinclude":
		dType = DirectiveSInclude
	case "ifeq":
		dType = DirectiveIfEq
	case "ifneq":
		dType = DirectiveIfNeq
	case "ifdef":
		dType = DirectiveIfDef
	case "ifndef":
		dType = DirectiveIfNDef
	case "else":
		dType = DirectiveElse
	case "endif":
		dType = DirectiveEndIf
	case "export":
		dType = DirectiveExport
	case "unexport":
		dType = DirectiveUnexport
	case "override":
		dType = DirectiveOverride
	case "vpath":
		dType = DirectiveVPath
	default:
		dType = DirectiveUnknown
	}

	return &Directive{
		Type:       dType,
		Args:       args,
		LineNumber: lineNum,
	}, nil
}

// SplitTokens splits a whitespace-separated string into tokens while preserving quoted strings
// and nested parenthesis/brace groups.
func SplitTokens(s string) []string {
	var tokens []string
	var current strings.Builder
	inParen := 0
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			current.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteByte(ch)
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteByte(ch)
			continue
		}
		if ch == '(' || ch == '{' {
			inParen++
			current.WriteByte(ch)
			continue
		}
		if ch == ')' || ch == '}' {
			if inParen > 0 {
				inParen--
			}
			current.WriteByte(ch)
			continue
		}

		if unicode.IsSpace(rune(ch)) && inParen == 0 && !inDoubleQuote && !inSingleQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// TrimRecipePrefix extracts Make execution modifiers ('@', '-', '+') from the recipe line.
func TrimRecipePrefix(recipe string) (prefix string, command string) {
	trimmed := strings.TrimSpace(recipe)
	var pre strings.Builder
	i := 0
	for i < len(trimmed) {
		ch := trimmed[i]
		if ch == '@' || ch == '-' || ch == '+' {
			pre.WriteByte(ch)
			i++
		} else {
			break
		}
	}
	return pre.String(), strings.TrimSpace(trimmed[i:])
}

// ParseRecipeLine parses a raw recipe line, extracting '@', '-', '+' modifiers and line number.
func ParseRecipeLine(raw string, lineNum int) RecipeLine {
	trimmed := strings.TrimLeft(raw, "\t ")
	cmd := trimmed
	silent := false
	ignoreErr := false
	alwaysExec := false

	for len(cmd) > 0 {
		switch cmd[0] {
		case '@':
			silent = true
			cmd = cmd[1:]
		case '-':
			ignoreErr = true
			cmd = cmd[1:]
		case '+':
			alwaysExec = true
			cmd = cmd[1:]
		default:
			goto DonePrefixes
		}
	}
DonePrefixes:
	cmd = strings.TrimSpace(cmd)
	return RecipeLine{
		Raw:         raw,
		Command:     cmd,
		Silent:      silent,
		IgnoreError: ignoreErr,
		AlwaysExec:  alwaysExec,
		LineNumber:  lineNum,
	}
}
