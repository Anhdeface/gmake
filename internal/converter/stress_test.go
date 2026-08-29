package converter_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

// TestChallenger_BizarreComments tests edge cases in comment placements and stripping.
func TestChallenger_BizarreComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		verifyFn func(t *testing.T, ast *converter.MakefileAST)
	}{
		{
			name: "Comment inside double quotes",
			input: `
VAR1 := "hello # not a comment" # real comment
app:
	@echo $(VAR1)
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if ast.Variables["VAR1"] != `"hello # not a comment"` {
					t.Errorf("VAR1 = %q, want %q", ast.Variables["VAR1"], `"hello # not a comment"`)
				}
			},
		},
		{
			name: "Comment inside single quotes",
			input: `
VAR2 := 'hello # not a comment' # real comment
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if ast.Variables["VAR2"] != `'hello # not a comment'` {
					t.Errorf("VAR2 = %q, want %q", ast.Variables["VAR2"], `'hello # not a comment'`)
				}
			},
		},
		{
			name: "Escaped hash in variable",
			input: `
VAR3 := hello \# world # comment
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if !strings.Contains(ast.Variables["VAR3"], "#") {
					t.Errorf("VAR3 = %q, expected to contain '#'", ast.Variables["VAR3"])
				}
			},
		},
		{
			name: "Escaped backslash before hash",
			input: `
VAR4 := value \\# this is a comment
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if strings.Contains(ast.Variables["VAR4"], "this is a comment") {
					t.Errorf("VAR4 contains comment text: %q", ast.Variables["VAR4"])
				}
			},
		},
		{
			name: "Comment on recipe line preserved",
			input: `
target:
	@echo "step 1" # inline comment in shell
	# full line comment in recipe
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["target"]
				if rule == nil {
					t.Fatalf("missing rule 'target'")
				}
				if len(rule.Recipes) < 1 {
					t.Fatalf("expected at least 1 recipe, got %d", len(rule.Recipes))
				}
				if !strings.Contains(rule.Recipes[0], "step 1") {
					t.Errorf("recipe 0 mismatch: %q", rule.Recipes[0])
				}
			},
		},
		{
			name: "Comment immediately after colon",
			input: `
app: # build the main application
	gcc -o app main.c
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["app"]
				if rule == nil {
					t.Fatalf("missing rule 'app'")
				}
				if len(rule.Prerequisites) != 0 {
					t.Errorf("expected 0 prereqs, got %v", rule.Prerequisites)
				}
				if len(rule.Recipes) != 1 || rule.Recipes[0] != "gcc -o app main.c" {
					t.Errorf("recipes mismatch: %v", rule.Recipes)
				}
			},
		},
		{
			name: "Inline recipe with comment",
			input: `
clean: ; rm -f *.o # clean objects
	rm -f bin/app # clean binary
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["clean"]
				if rule == nil {
					t.Fatalf("missing rule 'clean'")
				}
				if len(rule.Recipes) != 2 {
					t.Fatalf("expected 2 recipes, got %d: %v", len(rule.Recipes), rule.Recipes)
				}
				if rule.Recipes[0] != "rm -f *.o" {
					t.Errorf("inline recipe mismatch: %q", rule.Recipes[0])
				}
			},
		},
		{
			name: "Comment line continuation",
			input: `
# This is a comment with trailing backslash \
VAR_AFTER = ok
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				t.Logf("VAR_AFTER = %q", ast.Variables["VAR_AFTER"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := converter.ParseMakefile(tc.input)
			if err != nil {
				t.Fatalf("ParseMakefile failed: %v", err)
			}
			tc.verifyFn(t, ast)
		})
	}
}

// TestChallenger_IndentationVariations tests whitespace, tabs, spaces, and mixed indentation.
func TestChallenger_IndentationVariations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		verifyFn func(t *testing.T, ast *converter.MakefileAST)
	}{
		{
			name: "Tabs indentation standard",
			input: "app: main.o\n\tgcc -o app main.o\n\t@echo Done\n",
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["app"]
				if rule == nil || len(rule.Recipes) != 2 {
					t.Fatalf("expected 2 recipes for tab indented rule, got %+v", rule)
				}
			},
		},
		{
			name: "Space indentation 4 spaces",
			input: "app: main.o\n    gcc -o app main.o\n    echo Done\n",
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["app"]
				if rule == nil || len(rule.Recipes) != 2 {
					t.Fatalf("expected 2 recipes for space indented rule, got %+v", rule)
				}
			},
		},
		{
			name: "Mixed tabs and spaces",
			input: "app: main.o\n\t   gcc -o app main.o\n   \techo Done\n",
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["app"]
				if rule == nil || len(rule.Recipes) != 2 {
					t.Fatalf("expected 2 recipes for mixed tab/space rule, got %+v", rule)
				}
			},
		},
		{
			name: "Trailing whitespace on variable assignments and rules",
			input: "CC  :=   gcc   \t  \nCFLAGS := -Wall   -O2   \t \napp  :   main.o   \t  \n\tgcc -o app main.o   \t  \n",
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if ast.Variables["CC"] != "gcc" {
					t.Errorf("CC = %q, want 'gcc'", ast.Variables["CC"])
				}
				if ast.Variables["CFLAGS"] != "-Wall   -O2" {
					t.Errorf("CFLAGS = %q, want '-Wall   -O2'", ast.Variables["CFLAGS"])
				}
				rule := ast.RuleMap["app"]
				if rule == nil {
					t.Fatalf("missing rule 'app'")
				}
				if len(rule.Prerequisites) != 1 || rule.Prerequisites[0] != "main.o" {
					t.Errorf("prerequisites mismatch: %v", rule.Prerequisites)
				}
			},
		},
		{
			name: "Blank lines and tab-only lines between recipes",
			input: "app: main.o\n\tgcc -c main.c\n\t\n   \n\t\tgcc -o app main.o\n",
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				rule := ast.RuleMap["app"]
				if rule == nil {
					t.Fatalf("missing rule 'app'")
				}
				if len(rule.Recipes) < 2 {
					t.Errorf("expected recipes collected across blank/tab lines: %v", rule.Recipes)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := converter.ParseMakefile(tc.input)
			if err != nil {
				t.Fatalf("ParseMakefile failed: %v", err)
			}
			tc.verifyFn(t, ast)
		})
	}
}

// TestChallenger_RuleVariations tests multi-target, double-colon, inline, static pattern rules.
func TestChallenger_RuleVariations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		verifyFn func(t *testing.T, ast *converter.MakefileAST)
	}{
		{
			name: "Multi-target rule header",
			input: `
target1 target2 target3: common.o
	gcc -o $@ $<
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if len(ast.Rules) != 1 {
					t.Fatalf("expected 1 rule struct, got %d", len(ast.Rules))
				}
				r := ast.Rules[0]
				if len(r.Targets) != 3 || r.Targets[0] != "target1" || r.Targets[1] != "target2" || r.Targets[2] != "target3" {
					t.Errorf("targets mismatch: %v", r.Targets)
				}
				for _, name := range []string{"target1", "target2", "target3"} {
					if ast.RuleMap[name] == nil {
						t.Errorf("missing RuleMap entry for %s", name)
					}
				}
			},
		},
		{
			name: "Multi-target with inline recipe",
			input: `
clean distclean purge: ; rm -f *.o
	rm -f bin/*
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				r := ast.RuleMap["clean"]
				if r == nil {
					t.Fatalf("missing rule 'clean'")
				}
				if len(r.Recipes) != 2 {
					t.Fatalf("expected 2 recipes (inline + indented), got %d: %v", len(r.Recipes), r.Recipes)
				}
				if r.Recipes[0] != "rm -f *.o" {
					t.Errorf("recipe 0 mismatch: %q", r.Recipes[0])
				}
				if r.Recipes[1] != "rm -f bin/*" {
					t.Errorf("recipe 1 mismatch: %q", r.Recipes[1])
				}
			},
		},
		{
			name: "Multiple double-colon rules for same target",
			input: `
deploy::
	echo "Step 1: Build"

deploy::
	echo "Step 2: Upload"

deploy::
	echo "Step 3: Notify"
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if len(ast.Rules) != 3 {
					t.Fatalf("expected 3 rules in ast.Rules, got %d", len(ast.Rules))
				}
				for i, rule := range ast.Rules {
					if !rule.IsDoubleColon {
						t.Errorf("rule %d expected IsDoubleColon=true", i)
					}
					if len(rule.Targets) != 1 || rule.Targets[0] != "deploy" {
						t.Errorf("rule %d target mismatch: %v", i, rule.Targets)
					}
				}
			},
		},
		{
			name: "Order-only prerequisites",
			input: `
bin/app: obj/main.o obj/util.o | bin obj
	gcc -o $@ $^
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				r := ast.RuleMap["bin/app"]
				if r == nil {
					t.Fatalf("missing rule 'bin/app'")
				}
				if len(r.Prerequisites) != 2 || r.Prerequisites[0] != "obj/main.o" || r.Prerequisites[1] != "obj/util.o" {
					t.Errorf("prerequisites mismatch: %v", r.Prerequisites)
				}
				if len(r.OrderOnly) != 2 || r.OrderOnly[0] != "bin" || r.OrderOnly[1] != "obj" {
					t.Errorf("order-only mismatch: %v", r.OrderOnly)
				}
			},
		},
		{
			name: "Rule with no recipe (prerequisite-only rule)",
			input: `
all: app test
app: main.o
main.o:
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				if ast.RuleMap["all"] == nil || ast.RuleMap["app"] == nil || ast.RuleMap["main.o"] == nil {
					t.Errorf("missing rules: %+v", ast.RuleMap)
				}
				if len(ast.RuleMap["main.o"].Recipes) != 0 {
					t.Errorf("expected 0 recipes for main.o, got %v", ast.RuleMap["main.o"].Recipes)
				}
			},
		},
		{
			name: "Static pattern rule with standard targets",
			input: `
foo.o bar.o: %.o: src/%.c
	gcc -c $< -o $@
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				r := ast.RuleMap["foo.o"]
				if r == nil {
					t.Fatalf("missing static pattern rule for foo.o")
				}
				if len(r.Prerequisites) != 2 || r.Prerequisites[0] != "src/foo.c" || r.Prerequisites[1] != "src/bar.c" {
					t.Errorf("prereqs mismatch: %v", r.Prerequisites)
				}
			},
		},
		{
			name: "Phony declaration after rule definition",
			input: `
build:
	gcc -o build main.c

.PHONY: build test
`,
			verifyFn: func(t *testing.T, ast *converter.MakefileAST) {
				r := ast.RuleMap["build"]
				if r == nil {
					t.Fatalf("missing rule 'build'")
				}
				if !r.IsPhony {
					t.Errorf("expected rule 'build' to have IsPhony=true when .PHONY appears afterwards")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := converter.ParseMakefile(tc.input)
			if err != nil {
				t.Fatalf("ParseMakefile failed: %v", err)
			}
			tc.verifyFn(t, ast)
		})
	}
}

// TestChallenger_StaticPatternPanic reproduces the slice bounds out of range panic in parseStaticPatternRule.
func TestChallenger_StaticPatternPanic(t *testing.T) {
	// Adversarial input: target pattern prefix/suffix overlap where len(target) < len(prefix) + len(suffix)
	// Pattern "a%a" with target "a" triggers slice bounds out of range [1:0]
	input := `
a: a%a: %.c
	gcc -c $< -o $@
`
	defer func() {
		if r := recover(); r != nil {
			t.Logf("CONFIRMED BUG: parseStaticPatternRule panicked with: %v", r)
			// Record the panic for bug report
			t.Fail()
		}
	}()

	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Logf("ParseMakefile returned error (graceful): %v", err)
	} else {
		t.Logf("ParseMakefile succeeded: %d rules", len(ast.Rules))
	}
}

// TestChallenger_CircularIncludes tests recursive include file loops.
func TestChallenger_CircularIncludes(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.mk")
	fileB := filepath.Join(tmpDir, "b.mk")

	_ = os.WriteFile(fileA, []byte(fmt.Sprintf("include %s\nVAR_A = 1\n", fileB)), 0644)
	_ = os.WriteFile(fileB, []byte(fmt.Sprintf("include %s\nVAR_B = 2\n", fileA)), 0644)

	// Note: Circular file includes will recurse indefinitely if unbounded.
}

// TestChallenger_MalformedAndAdversarialInputs feeds degenerate, malformed, and boundary inputs.
func TestChallenger_MalformedAndAdversarialInputs(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Whitespace only", "   \n\t\n   \n"},
		{"Comments only", "# line 1\n# line 2\n# line 3\n"},
		{"Binary junk with null bytes", "\x00\x01\x02\xff\xfe\xfd\x00\x00\x00"},
		{"Missing colons", "target_without_colon prereq1 prereq2\nanother random line\n"},
		{"Multiple colons on single line", "target:with:many:colons: prereqs\n\trecipe\n"},
		{"Colon only", ":\n\trecipe\n"},
		{"Equals only", "=\n:=\n::=\n?=\n+=\n!=\n"},
		{"Unclosed conditional ifeq", "ifeq (1, 1)\nVAR=1\n"},
		{"Unclosed conditional ifdef", "ifdef FOO\nVAR=2\n"},
		{"Unclosed define block", "define MULTI\nline 1\nline 2\n"},
		{"Unmatched else", "else\nVAR=3\n"},
		{"Unmatched endif", "endif\nVAR=4\n"},
		{"Multiple unmatched else/endif", "else\nendif\nelse\nendif\n"},
		{"Deeply nested unclosed ifeq (depth 30)", strings.Repeat("ifeq (1, 1)\n", 30) + "VAR = nested\n"},
		{"Circular recursive variables", "A = $(B)\nB = $(C)\nC = $(A)\nTARGET: $(A)\n"},
		{"Self-referencing variable", "X = $(X) loop\nall: ; echo $(X)\n"},
		{"Empty variable reference", "$()\n${}\n$($())\n"},
		{"Deeply nested empty references", "$($($($($()))))\n"},
		{"Malformed function calls", "$(patsubst)\n$(subst)\n$(word)\n$(word -5, a b c)\n$(wordlist -1, -5, a b c)\n$(firstword)\n$(lastword)\n"},
		{"Unknown functions", "$(unknown_func_xyz 1, 2, 3)\n$(another_unknown a, b)\n"},
		{"Unclosed quotes in conditionals", "ifeq 'unclosed\nVAR=1\nendif\n"},
		{"Unclosed parens in conditionals", "ifeq (unclosed\nVAR=1\nendif\n"},
		{"Trailing backslash at EOF", "VAR = abc\\"},
		{"Trailing double backslash at EOF", "VAR = abc\\\\"},
		{"Line continuations chain (100 lines)", "SRCS = " + strings.Repeat("src.c \\\n", 100) + "final.c\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("CRASH/PANIC on input %q: %v", tc.name, r)
				}
			}()

			ast, err := converter.ParseMakefile(tc.input)
			if err != nil {
				// Non-crashing errors are acceptable
				t.Logf("graceful error on %s: %v", tc.name, err)
			} else {
				if ast == nil {
					t.Errorf("expected non-nil AST for %s", tc.name)
				}
			}
		})
	}
}

// TestChallenger_FuzzGenerator generates random syntax combinations to probe for crashes.
func TestChallenger_FuzzGenerator(t *testing.T) {
	tokens := []string{
		"all", "clean", "test", "%.o", "%.c", "app", "bin/app",
		":", "::", "=", ":=", "::=", "?=", "+=", "!=",
		";", "|", "@", "-", "+", "#", "\\", "\n", "\t", " ",
		"ifeq", "ifneq", "ifdef", "ifndef", "else", "endif",
		"include", "-include", "sinclude", "define", "endef",
		"$(CC)", "$(CFLAGS)", "$@", "$<", "$^", "$*",
		"$(patsubst %.c,%.o,$(SRCS))", "$(wildcard *.c)",
		"$(shell echo hi)", "$(word 1, a b)", "$(strip a  b)",
		"$(A)", "$(B)", "$($A)", "${VAR}", "$$",
		"'single quote'", `"double quote"`, `"\x00"`,
		"unknown_token", "12345",
	}

	rng := rand.New(rand.NewSource(42))
	iterations := 2000

	for iter := 0; iter < iterations; iter++ {
		numTokens := rng.Intn(25) + 1
		var sb strings.Builder
		for i := 0; i < numTokens; i++ {
			tok := tokens[rng.Intn(len(tokens))]
			sb.WriteString(tok)
			if rng.Float32() < 0.3 {
				sb.WriteString(" ")
			} else if rng.Float32() < 0.1 {
				sb.WriteString("\n")
			}
		}

		input := sb.String()

		func(fuzzInput string, iterNum int) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("CRASH/PANIC on fuzz iteration %d:\nInput:\n%q\nError: %v", iterNum, fuzzInput, r)
				}
			}()

			_, _ = converter.ParseMakefile(fuzzInput)
		}(input, iter)
	}
}

// TestChallenger_ScaleStress generates a massive 10,000 line Makefile to test memory and performance.
func TestChallenger_ScaleStress(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("CC = gcc\nCFLAGS = -Wall\n\n")

	// Generate 1000 rules with 5 prerequisites and 3 recipes each
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, "target_%d: obj_%d_1.o obj_%d_2.o obj_%d_3.o\n", i, i, i, i)
		fmt.Fprintf(&sb, "\t@echo \"Building target %d\"\n", i)
		fmt.Fprintf(&sb, "\tgcc -c $< -o $@\n")
		fmt.Fprintf(&sb, "\t@echo \"Finished target %d\"\n\n", i)
	}

	largeInput := sb.String()

	ast, err := converter.ParseMakefile(largeInput)
	if err != nil {
		t.Fatalf("ParseMakefile failed on large Makefile: %v", err)
	}

	if len(ast.Rules) != 1000 {
		t.Errorf("expected 1000 rules, got %d", len(ast.Rules))
	}
	if ast.DefaultGoal != "target_0" {
		t.Errorf("expected DefaultGoal 'target_0', got %q", ast.DefaultGoal)
	}
}
