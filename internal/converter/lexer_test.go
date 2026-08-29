package converter_test

import (
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

func TestNormalizeLines_Continuations(t *testing.T) {
	input := "SRCS = main.c \\\n\tutil.c \\\n\thelper.c\nCC = gcc\n"
	lines := converter.NormalizeLines(input)

	if len(lines) != 2 {
		t.Fatalf("expected 2 logical lines, got %d", len(lines))
	}

	first := lines[0]
	if first.StartLine != 1 || first.EndLine != 3 {
		t.Errorf("expected line 1..3, got %d..%d", first.StartLine, first.EndLine)
	}

	expectedText := "SRCS = main.c util.c helper.c"
	if first.Text != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, first.Text)
	}

	second := lines[1]
	if second.StartLine != 4 || second.EndLine != 4 {
		t.Errorf("expected line 4..4, got %d..%d", second.StartLine, second.EndLine)
	}
	if second.Text != "CC = gcc" {
		t.Errorf("expected text %q, got %q", "CC = gcc", second.Text)
	}
}

func TestNormalizeLines_EscapedBackslashes(t *testing.T) {
	// Even number of trailing backslashes is an escaped backslash, not a continuation
	input := "VAR = line with double backslash\\\\\nNEXT = 1\n"
	lines := converter.NormalizeLines(input)

	if len(lines) != 2 {
		t.Fatalf("expected 2 logical lines, got %d", len(lines))
	}
	if lines[0].StartLine != 1 || lines[0].EndLine != 1 {
		t.Errorf("expected single line for double backslash, got %d..%d", lines[0].StartLine, lines[0].EndLine)
	}
}

func TestNormalizeLines_CRLF(t *testing.T) {
	input := "A = 1\r\nB = 2\rC = 3\n"
	lines := converter.NormalizeLines(input)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Text != "A = 1" || lines[1].Text != "B = 2" || lines[2].Text != "C = 3" {
		t.Errorf("CRLF normalization failed: %+v", lines)
	}
}

func TestStripComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain comment",
			input:    "CFLAGS = -Wall # compiler flags",
			expected: "CFLAGS = -Wall ",
		},
		{
			name:     "hash in double quotes",
			input:    `MSG = "Hello # World" # comment`,
			expected: `MSG = "Hello # World" `,
		},
		{
			name:     "hash in single quotes",
			input:    "MSG = 'Hello # World' # comment",
			expected: "MSG = 'Hello # World' ",
		},
		{
			name:     "escaped hash",
			input:    `MSG = Hello \# World # comment`,
			expected: `MSG = Hello \# World `,
		},
		{
			name:     "full line comment",
			input:    "# this is a comment",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := converter.StripComments(tc.input)
			if res != tc.expected {
				t.Errorf("StripComments(%q) = %q; want %q", tc.input, res, tc.expected)
			}
		})
	}
}

func TestClassifyLine(t *testing.T) {
	tests := []struct {
		name          string
		line          *converter.LogicalLine
		inRuleContext bool
		expected      converter.LineType
	}{
		{
			name: "blank line",
			line: &converter.LogicalLine{
				Raw:  "   ",
				Text: "   ",
			},
			inRuleContext: false,
			expected:      converter.LineTypeBlank,
		},
		{
			name: "comment line",
			line: &converter.LogicalLine{
				Raw:  "# comment",
				Text: "# comment",
			},
			inRuleContext: false,
			expected:      converter.LineTypeBlank,
		},
		{
			name: "recipe line tab",
			line: &converter.LogicalLine{
				Raw:        "\tgcc -c main.c",
				Text:       "\tgcc -c main.c",
				LeadingTab: true,
			},
			inRuleContext: true,
			expected:      converter.LineTypeRecipe,
		},
		{
			name: "recipe line space in rule context",
			line: &converter.LogicalLine{
				Raw:           "    gcc -c main.c",
				Text:          "    gcc -c main.c",
				LeadingSpaces: 4,
			},
			inRuleContext: true,
			expected:      converter.LineTypeRecipe,
		},
		{
			name: "variable assignment",
			line: &converter.LogicalLine{
				Raw:  "CC := gcc",
				Text: "CC := gcc",
			},
			inRuleContext: false,
			expected:      converter.LineTypeVariable,
		},
		{
			name: "directive include",
			line: &converter.LogicalLine{
				Raw:  "include defs.mk",
				Text: "include defs.mk",
			},
			inRuleContext: false,
			expected:      converter.LineTypeDirective,
		},
		{
			name: "rule header",
			line: &converter.LogicalLine{
				Raw:  "app: main.o",
				Text: "app: main.o",
			},
			inRuleContext: false,
			expected:      converter.LineTypeRule,
		},
		{
			name: "malformed line",
			line: &converter.LogicalLine{
				Raw:  "just some random text without colon or equals",
				Text: "just some random text without colon or equals",
			},
			inRuleContext: false,
			expected:      converter.LineTypeMalformed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := converter.ClassifyLine(tc.line, tc.inRuleContext)
			if got != tc.expected {
				t.Errorf("ClassifyLine() = %v (%s); want %v (%s)", got, got.String(), tc.expected, tc.expected.String())
			}
		})
	}
}

func TestParseVariableAssignment(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVal     string
		wantFlavor  converter.AssignmentFlavor
		wantExport  bool
		wantOver    bool
		wantErr     bool
	}{
		{"CC = gcc", "CC", "gcc", converter.FlavorRecursive, false, false, false},
		{"CFLAGS := -Wall -O2", "CFLAGS", "-Wall -O2", converter.FlavorSimple, false, false, false},
		{"CXX ::= g++", "CXX", "g++", converter.FlavorSimplePOSIX, false, false, false},
		{"DEBUG ?= 1", "DEBUG", "1", converter.FlavorConditional, false, false, false},
		{"LIBS += -lm", "LIBS", "-lm", converter.FlavorAppend, false, false, false},
		{"GIT_REV != git rev-parse HEAD", "GIT_REV", "git rev-parse HEAD", converter.FlavorShell, false, false, false},
		{"export MY_VAR = hello", "MY_VAR", "hello", converter.FlavorRecursive, true, false, false},
		{"override OPT = -O3", "OPT", "-O3", converter.FlavorRecursive, false, true, false},
		{"not_an_assignment", "", "", "", false, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			res, err := converter.ParseVariableAssignment(tc.input, 1)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if res.Name != tc.wantName || res.Value != tc.wantVal || res.Flavor != tc.wantFlavor ||
				res.Exported != tc.wantExport || res.Override != tc.wantOver {
				t.Errorf("ParseVariableAssignment(%q) = %+v; want name=%s, val=%s, flavor=%s, exp=%v, over=%v",
					tc.input, res, tc.wantName, tc.wantVal, tc.wantFlavor, tc.wantExport, tc.wantOver)
			}
		})
	}
}

func TestParseRuleHeader(t *testing.T) {
	tests := []struct {
		input         string
		wantTargets   []string
		wantPrereqs   []string
		wantInline    string
		wantDouble    bool
		wantPattern   bool
		wantErr       bool
	}{
		{
			input:       "app: main.o util.o",
			wantTargets: []string{"app"},
			wantPrereqs: []string{"main.o", "util.o"},
			wantDouble:  false,
			wantPattern: false,
		},
		{
			input:       "app1 app2: common.o",
			wantTargets: []string{"app1", "app2"},
			wantPrereqs: []string{"common.o"},
			wantDouble:  false,
			wantPattern: false,
		},
		{
			input:       "%.o: %.c",
			wantTargets: []string{"%.o"},
			wantPrereqs: []string{"%.c"},
			wantDouble:  false,
			wantPattern: true,
		},
		{
			input:       "clean:: ; rm -f *.o",
			wantTargets: []string{"clean"},
			wantPrereqs: nil,
			wantInline:  "rm -f *.o",
			wantDouble:  true,
			wantPattern: false,
		},
		{
			input:       "$(TARGET): $(OBJS)",
			wantTargets: []string{"$(TARGET)"},
			wantPrereqs: []string{"$(OBJS)"},
			wantDouble:  false,
			wantPattern: false,
		},
		{
			input:   "no_colon_here",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			targets, prereqs, inline, isDouble, isPat, err := converter.ParseRuleHeader(tc.input, 1)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if len(targets) != len(tc.wantTargets) {
				t.Errorf("targets mismatch: got %v, want %v", targets, tc.wantTargets)
			}
			if len(prereqs) != len(tc.wantPrereqs) {
				t.Errorf("prereqs mismatch: got %v, want %v", prereqs, tc.wantPrereqs)
			}
			if inline != tc.wantInline {
				t.Errorf("inline mismatch: got %q, want %q", inline, tc.wantInline)
			}
			if isDouble != tc.wantDouble {
				t.Errorf("isDouble mismatch: got %v, want %v", isDouble, tc.wantDouble)
			}
			if isPat != tc.wantPattern {
				t.Errorf("isPattern mismatch: got %v, want %v", isPat, tc.wantPattern)
			}
		})
	}
}

func TestSplitTokens(t *testing.T) {
	input := `bin/app "my lib/lib.a" 'another lib' $(patsubst %.c,%.o,$(SRCS))`
	tokens := converter.SplitTokens(input)

	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "bin/app" {
		t.Errorf("token 0 mismatch: %q", tokens[0])
	}
	if tokens[1] != `"my lib/lib.a"` {
		t.Errorf("token 1 mismatch: %q", tokens[1])
	}
	if tokens[2] != `'another lib'` {
		t.Errorf("token 2 mismatch: %q", tokens[2])
	}
	if tokens[3] != "$(patsubst %.c,%.o,$(SRCS))" {
		t.Errorf("token 3 mismatch: %q", tokens[3])
	}
}

func TestTrimRecipePrefix(t *testing.T) {
	tests := []struct {
		input      string
		wantPrefix string
		wantCmd    string
	}{
		{"@echo Building", "@", "echo Building"},
		{"-rm -f *.o", "-", "rm -f *.o"},
		{"+make -C subdir", "+", "make -C subdir"},
		{"@-echo Silent ignore error", "@-", "echo Silent ignore error"},
		{"+@mkdir -p bin", "+@", "mkdir -p bin"},
		{"gcc -o app main.c", "", "gcc -o app main.c"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pre, cmd := converter.TrimRecipePrefix(tc.input)
			if pre != tc.wantPrefix || cmd != tc.wantCmd {
				t.Errorf("TrimRecipePrefix(%q) = (%q, %q); want (%q, %q)", tc.input, pre, cmd, tc.wantPrefix, tc.wantCmd)
			}
		})
	}
}

func TestParseRecipeLine(t *testing.T) {
	rec := converter.ParseRecipeLine("\t@-mkdir -p $(dir $@)", 10)
	if !rec.Silent {
		t.Errorf("expected Silent = true")
	}
	if !rec.IgnoreError {
		t.Errorf("expected IgnoreError = true")
	}
	if rec.AlwaysExec {
		t.Errorf("expected AlwaysExec = false")
	}
	if rec.Command != "mkdir -p $(dir $@)" {
		t.Errorf("command mismatch: %q", rec.Command)
	}
	if rec.LineNumber != 10 {
		t.Errorf("line number mismatch: %d", rec.LineNumber)
	}
}
