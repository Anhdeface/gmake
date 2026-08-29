package converter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

func TestParseMakefile_VariableAssignments(t *testing.T) {
	input := `
# Simple immediate assignment := and ::=
A := hello
B := $(A) world
C ::= $(B) !

# Recursive lazy assignment =
D = $(E) deferred
E = lazy

# Conditional default assignment ?=
F = initial
F ?= overridden
G ?= set_default

# Append assignment +=
H := -Wall
H += -O2
I = first
I += second

# Shell assignment !=
SH != echo "from_shell"
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ast.Variables["A"] != "hello" {
		t.Errorf("A = %q, want 'hello'", ast.Variables["A"])
	}
	if ast.Variables["B"] != "hello world" {
		t.Errorf("B = %q, want 'hello world'", ast.Variables["B"])
	}
	if ast.Variables["C"] != "hello world !" {
		t.Errorf("C = %q, want 'hello world !'", ast.Variables["C"])
	}
	if ast.Variables["D"] != "lazy deferred" {
		t.Errorf("D = %q, want 'lazy deferred'", ast.Variables["D"])
	}
	if ast.Variables["F"] != "initial" {
		t.Errorf("F = %q, want 'initial'", ast.Variables["F"])
	}
	if ast.Variables["G"] != "set_default" {
		t.Errorf("G = %q, want 'set_default'", ast.Variables["G"])
	}
	if ast.Variables["H"] != "-Wall -O2" {
		t.Errorf("H = %q, want '-Wall -O2'", ast.Variables["H"])
	}
	if ast.Variables["I"] != "first second" {
		t.Errorf("I = %q, want 'first second'", ast.Variables["I"])
	}
	if ast.Variables["SH"] != "from_shell" {
		t.Errorf("SH = %q, want 'from_shell'", ast.Variables["SH"])
	}
}

func TestParseMakefile_VariableExpansionsAndSubstitutions(t *testing.T) {
	input := `
PLATFORM = linux
SRC_linux = main.c util.c

# Nested variable dereferencing
SRCS := $($(PLATFORM)_SRC)
SRCS_ALT := ${SRC_${PLATFORM}}

# Suffix substitution
OBJS := $(SRCS_ALT:.c=.o)

# Pattern substitution
SRC_PATHS := $(OBJS:%.o=src/%.c)

# Escaped dollar
DOLLAR_VAL := $$HOME
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ast.Variables["SRCS_ALT"] != "main.c util.c" {
		t.Errorf("SRCS_ALT = %q, want 'main.c util.c'", ast.Variables["SRCS_ALT"])
	}
	if ast.Variables["OBJS"] != "main.o util.o" {
		t.Errorf("OBJS = %q, want 'main.o util.o'", ast.Variables["OBJS"])
	}
	if ast.Variables["SRC_PATHS"] != "src/main.c src/util.c" {
		t.Errorf("SRC_PATHS = %q, want 'src/main.c src/util.c'", ast.Variables["SRC_PATHS"])
	}
	if ast.Variables["DOLLAR_VAL"] != "$HOME" {
		t.Errorf("DOLLAR_VAL = %q, want '$HOME'", ast.Variables["DOLLAR_VAL"])
	}
}

func TestParseMakefile_CycleDetection(t *testing.T) {
	input := `
A = $(B)
B = $(A)
TARGET = $(A)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile should not crash on cycles: %v", err)
	}
	if ast.Variables["A"] != "" || ast.Variables["B"] != "" {
		t.Errorf("cyclical variables should safely evaluate to empty string; got A=%q, B=%q", ast.Variables["A"], ast.Variables["B"])
	}
	if len(ast.Warnings) == 0 {
		t.Errorf("expected warning logged for cyclic dependency")
	}
}

func TestParseMakefile_BuiltinFunctions(t *testing.T) {
	input := `
SRCS = src/main.c src/util.c lib/algo.c test/test_main.c test/test_algo.c

PAT_RES := $(patsubst src/%.c, obj/%.o, src/a.c src/b.c)
SUBST_RES := $(subst .c,.o, a.c b.c)
PREFIX_RES := $(addprefix -I, include src/include)
SUFFIX_RES := $(addsuffix .a, lib/libx lib/liby)
DIR_RES := $(dir src/foo.c bar.c)
NOTDIR_RES := $(notdir src/foo.c bar.c)
BASE_RES := $(basename src/foo.c.bak bar.o)
SUF_RES := $(suffix src/foo.c.bak bar.o baz)
FILTER_RES := $(filter test/%, $(SRCS))
FILTEROUT_RES := $(filter-out test/%, $(SRCS))
STRIP_RES := $(strip   a   b    c   )
FIRST_RES := $(firstword a b c)
LAST_RES := $(lastword a b c)
WORD_RES := $(word 2, a b c d)
WORDS_RES := $(words a b c d)
WORDLIST_RES := $(wordlist 2, 4, a b c d e)
SORT_RES := $(sort b a c b a)
JOIN_RES := $(join a b c, .c .h .o)
IF_TRUE := $(if non_empty, then_val, else_val)
IF_FALSE := $(if , then_val, else_val)
OR_RES := $(or , , first_non_empty, second)
AND_RES := $(and 1, 2, 3)
FOREACH_RES := $(foreach dir, src lib test, $(dir)/main.c)
ORIGIN_CC := $(origin CC)
FLAVOR_CC := $(flavor CC)

CUSTOM_MACRO = hello $(1) and $(2)
CALL_RES := $(call CUSTOM_MACRO, alice, bob)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := map[string]string{
		"PAT_RES":       "obj/a.o obj/b.o",
		"SUBST_RES":     "a.o b.o",
		"PREFIX_RES":    "-Iinclude -Isrc/include",
		"SUFFIX_RES":    "lib/libx.a lib/liby.a",
		"DIR_RES":       "src/ ./",
		"NOTDIR_RES":    "foo.c bar.c",
		"BASE_RES":      "src/foo.c bar",
		"SUF_RES":       ".bak .o",
		"FILTER_RES":    "test/test_main.c test/test_algo.c",
		"FILTEROUT_RES": "src/main.c src/util.c lib/algo.c",
		"STRIP_RES":     "a b c",
		"FIRST_RES":     "a",
		"LAST_RES":      "c",
		"WORD_RES":      "b",
		"WORDS_RES":     "4",
		"WORDLIST_RES":  "b c d",
		"SORT_RES":      "a b c",
		"JOIN_RES":      "a.c b.h c.o",
		"IF_TRUE":       "then_val",
		"IF_FALSE":      "else_val",
		"OR_RES":        "first_non_empty",
		"AND_RES":       "3",
		"FOREACH_RES":   "src/main.c lib/main.c test/main.c",
		"CALL_RES":      "hello alice and bob",
		"ORIGIN_CC":     "default",
		"FLAVOR_CC":     "recursive",
	}

	for varName, want := range tests {
		got := ast.Variables[varName]
		if got != want {
			t.Errorf("Variable %s = %q; want %q", varName, got, want)
		}
	}
}

func TestParseMakefile_MultiLineDefine(t *testing.T) {
	input := `
define BANNER
================================
Building Application
================================
endef

define RUN_CMD
	@echo "Starting..."
	./bin/app --verbose
endef

app:
	@echo "$(BANNER)"
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(ast.Variables["BANNER"], "Building Application") {
		t.Errorf("BANNER variable mismatch: %q", ast.Variables["BANNER"])
	}
	if !strings.Contains(ast.Variables["RUN_CMD"], "./bin/app --verbose") {
		t.Errorf("RUN_CMD variable mismatch: %q", ast.Variables["RUN_CMD"])
	}
}

func TestParseMakefile_Conditionals(t *testing.T) {
	input := `
DEBUG = 1
OS = linux

ifeq ($(DEBUG), 1)
CFLAGS += -g -O0
else
CFLAGS += -O3
endif

ifneq ($(OS), windows)
LDFLAGS += -lpthread
else
LDFLAGS += -lws2_32
endif

ifdef UNDEFINED_VAR
SHOULD_NOT_BE_SET = 1
else
SHOULD_BE_SET = 1
endif

ifndef UNSET_VAR
UNSET_CHECK = ok
endif

# Nested conditional
MODE = release
ifeq ($(DEBUG), 1)
  ifeq ($(MODE), release)
    COMBO = debug_release
  else
    COMBO = debug_dev
  endif
endif
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ast.Variables["CFLAGS"] != "-g -O0" {
		t.Errorf("CFLAGS = %q, want '-g -O0'", ast.Variables["CFLAGS"])
	}
	if ast.Variables["LDFLAGS"] != "-lpthread" {
		t.Errorf("LDFLAGS = %q, want '-lpthread'", ast.Variables["LDFLAGS"])
	}
	if ast.Variables["SHOULD_BE_SET"] != "1" || ast.Variables["SHOULD_NOT_BE_SET"] != "" {
		t.Errorf("ifdef/else evaluation failed: SHOULD_BE_SET=%q, SHOULD_NOT_BE_SET=%q",
			ast.Variables["SHOULD_BE_SET"], ast.Variables["SHOULD_NOT_BE_SET"])
	}
	if ast.Variables["UNSET_CHECK"] != "ok" {
		t.Errorf("UNSET_CHECK = %q, want 'ok'", ast.Variables["UNSET_CHECK"])
	}
	if ast.Variables["COMBO"] != "debug_release" {
		t.Errorf("COMBO = %q, want 'debug_release'", ast.Variables["COMBO"])
	}
}

func TestParseMakefile_RulesAndRecipes(t *testing.T) {
	input := `
.PHONY: all clean test

TARGET = bin/app
OBJS = src/main.o src/util.o

all: $(TARGET)

$(TARGET): $(OBJS) | bin
	@mkdir -p bin
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

# Multi-target rule
clean distclean: ; @rm -f $(OBJS) $(TARGET)
	@echo "Cleaned"

test: $(TARGET)
	+./bin/app --test
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. DefaultGoal
	if ast.DefaultGoal != "bin/app" && ast.DefaultGoal != "all" {
		// all is first target in AST
		t.Logf("DefaultGoal: %q", ast.DefaultGoal)
	}
	if ast.RuleMap["all"] == nil {
		t.Fatalf("missing rule 'all'")
	}
	if !ast.RuleMap["all"].IsPhony {
		t.Errorf("expected 'all' rule to be marked IsPhony=true")
	}

	// 2. Target bin/app
	appRule := ast.RuleMap["bin/app"]
	if appRule == nil {
		t.Fatalf("missing rule 'bin/app'")
	}
	if len(appRule.Prerequisites) != 2 || appRule.Prerequisites[0] != "src/main.o" || appRule.Prerequisites[1] != "src/util.o" {
		t.Errorf("prerequisites mismatch: %v", appRule.Prerequisites)
	}
	if len(appRule.OrderOnly) != 1 || appRule.OrderOnly[0] != "bin" {
		t.Errorf("order-only prerequisites mismatch: %v", appRule.OrderOnly)
	}
	if len(appRule.Recipes) != 2 {
		t.Fatalf("expected 2 recipes for bin/app, got %d: %v", len(appRule.Recipes), appRule.Recipes)
	}
	if !appRule.RecipeLines[0].Silent {
		t.Errorf("expected silent prefix '@' on recipe 0")
	}

	// 3. Pattern rule %.o: %.c
	if len(ast.PatternRules) != 1 {
		t.Fatalf("expected 1 pattern rule, got %d", len(ast.PatternRules))
	}
	patRule := ast.PatternRules[0]
	if patRule.Targets[0] != "%.o" || patRule.Prerequisites[0] != "%.c" || !patRule.IsPattern {
		t.Errorf("pattern rule mismatch: %+v", patRule)
	}

	// 4. Multi-target rule with inline recipe
	cleanRule := ast.RuleMap["clean"]
	distcleanRule := ast.RuleMap["distclean"]
	if cleanRule == nil || distcleanRule == nil || cleanRule != distcleanRule {
		t.Errorf("multi-target rule map pointers mismatch")
	}
	if !cleanRule.IsPhony || !distcleanRule.IsPhony {
		t.Errorf("clean and distclean should be marked IsPhony=true")
	}
	if len(cleanRule.Recipes) != 2 {
		t.Errorf("expected 2 recipes for clean rule (inline + indented), got %d: %v", len(cleanRule.Recipes), cleanRule.Recipes)
	}

	// 5. Test rule with '+' prefix
	testRule := ast.RuleMap["test"]
	if testRule == nil || len(testRule.RecipeLines) != 1 || !testRule.RecipeLines[0].AlwaysExec {
		t.Errorf("expected AlwaysExec '+' on test rule recipe: %+v", testRule)
	}
}

func TestParseMakefile_StaticPatternRules(t *testing.T) {
	input := `
OBJS = foo.o bar.o

all: $(OBJS)

$(OBJS): %.o: src/%.c
	$(CC) -c $< -o $@
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := ast.RuleMap["foo.o"]
	if rule == nil {
		t.Fatalf("missing static pattern rule for 'foo.o'")
	}
	if len(rule.Targets) != 2 || rule.Targets[0] != "foo.o" || rule.Targets[1] != "bar.o" {
		t.Errorf("targets mismatch: %v", rule.Targets)
	}
	if len(rule.Prerequisites) != 2 || rule.Prerequisites[0] != "src/foo.c" || rule.Prerequisites[1] != "src/bar.c" {
		t.Errorf("prerequisites mismatch: %v", rule.Prerequisites)
	}
}

func TestParseMakefile_DoubleColonRules(t *testing.T) {
	input := `
clean::
	rm -f *.o

clean::
	rm -f bin/app
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ast.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(ast.Rules))
	}
	if !ast.Rules[0].IsDoubleColon || !ast.Rules[1].IsDoubleColon {
		t.Errorf("expected both rules to have IsDoubleColon=true")
	}
}

func TestParseMakefile_IncludeDirectives(t *testing.T) {
	tmpDir := t.TempDir()
	incFile := filepath.Join(tmpDir, "inc.mk")
	incContent := "INC_VAR = from_included_file\ninc_target:\n\t@echo inc\n"
	if err := os.WriteFile(incFile, []byte(incContent), 0644); err != nil {
		t.Fatalf("failed to write tmp include file: %v", err)
	}

	input := `
include ` + incFile + `
-include missing_optional.mk
include nonexistent_mandatory.mk

main_target: inc_target
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile should not crash on include files: %v", err)
	}

	if ast.Variables["INC_VAR"] != "from_included_file" {
		t.Errorf("INC_VAR = %q, want 'from_included_file'", ast.Variables["INC_VAR"])
	}
	if ast.RuleMap["inc_target"] == nil {
		t.Errorf("missing inc_target rule from included file")
	}
	if len(ast.Warnings) == 0 {
		t.Errorf("expected warning for missing nonexistent_mandatory.mk")
	}
}

func TestParseMakefile_MalformedAndComplexGracefulDegradation(t *testing.T) {
	input := `
# Malformed lines with no colons or operators
just random unformatted text
1234567890 !@#$%^&*()

# Unclosed conditional
ifeq (1, 1)
VALID_VAR = ok
# Missing endif at EOF

# Unrecognized directive
custom_plugin_directive some args

# Empty lines and tabs
		
	

clean:
	@echo "Clean"
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("should not crash on malformed makefiles: %v", err)
	}

	if ast.Variables["VALID_VAR"] != "ok" {
		t.Errorf("VALID_VAR = %q, want 'ok'", ast.Variables["VALID_VAR"])
	}
	if ast.RuleMap["clean"] == nil {
		t.Errorf("clean rule should be parsed despite malformed lines")
	}
	if len(ast.Warnings) == 0 {
		t.Errorf("expected warnings recorded for malformed lines")
	}
}

func TestParseMakefile_Empty(t *testing.T) {
	ast, err := converter.ParseMakefile("")
	if err != nil {
		t.Fatalf("unexpected error for empty Makefile: %v", err)
	}
	if len(ast.Rules) != 0 || len(ast.Variables) == 0 {
		// default built-in variables are populated
		if ast.Variables["CC"] != "gcc" {
			t.Errorf("default CC should be gcc")
		}
	}
}
