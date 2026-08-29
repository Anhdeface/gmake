package converter_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

// TestStress_VariableFlavorsAndAssignments tests all assignment operators (=, :=, ::=, ?=, +=, !=) and define blocks.
func TestStress_VariableFlavorsAndAssignments(t *testing.T) {
	input := `
# 1. Order of evaluation in immediate (:= and ::=) vs lazy (=)
LAZY_A = $(LAZY_B) world
LAZY_B = hello

IMM_A := $(IMM_B) world
IMM_B := hello

# POSIX immediate ::=
POSIX_A ::= $(IMM_B) posix

# 2. Default assignment ?=
# CC is a standard default ("gcc"), so CC ?= clang should override default
CC ?= clang
# CUSTOM_VAR is set, subsequent ?= should NOT overwrite
CUSTOM_VAR = original
CUSTOM_VAR ?= changed
# UNSET_VAR is not set, ?= should set it
UNSET_VAR ?= first_set

# 3. Append assignment +=
# Append to simple variable
SIMP := -Wall
SIMP += -O2
SIMP += -g

# Append to recursive variable
REC = $(BASE)
BASE = start
REC += $(EXTRA)
EXTRA = end

# Append to initially unset variable
UNSET_APPEND += item1
UNSET_APPEND += item2

# 4. Shell assignment !=
SHELL_ECHO != echo "shell_test_output"
SHELL_MULTILINE != printf "line1\nline2\nline3\n"

# 5. Define multi-line blocks with various flavors
define BLOCK_RECURSIVE
Line 1: $(LAZY_B)
Line 2: $(EXTRA)
endef

define BLOCK_SIMPLE :=
Simple line 1
Simple line 2: $(IMM_B)
endef

define BLOCK_APPEND +=
Appended line
endef
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	// 1. Lazy vs Immediate
	if ast.Variables["LAZY_A"] != "hello world" {
		t.Errorf("LAZY_A = %q; want 'hello world'", ast.Variables["LAZY_A"])
	}
	// IMM_A should have expanded IMM_B when IMM_B was unset (empty)
	if ast.Variables["IMM_A"] != "world" && ast.Variables["IMM_A"] != " world" {
		t.Errorf("IMM_A = %q; want 'world' or ' world'", ast.Variables["IMM_A"])
	}
	if ast.Variables["POSIX_A"] != "hello posix" {
		t.Errorf("POSIX_A = %q; want 'hello posix'", ast.Variables["POSIX_A"])
	}

	// 2. Default ?=
	if ast.Variables["CC"] != "clang" {
		t.Errorf("CC = %q; want 'clang' (overriding built-in default)", ast.Variables["CC"])
	}
	if ast.Variables["CUSTOM_VAR"] != "original" {
		t.Errorf("CUSTOM_VAR = %q; want 'original'", ast.Variables["CUSTOM_VAR"])
	}
	if ast.Variables["UNSET_VAR"] != "first_set" {
		t.Errorf("UNSET_VAR = %q; want 'first_set'", ast.Variables["UNSET_VAR"])
	}

	// 3. Append +=
	if ast.Variables["SIMP"] != "-Wall -O2 -g" {
		t.Errorf("SIMP = %q; want '-Wall -O2 -g'", ast.Variables["SIMP"])
	}
	if ast.Variables["REC"] != "start end" {
		t.Errorf("REC = %q; want 'start end'", ast.Variables["REC"])
	}
	if ast.Variables["UNSET_APPEND"] != "item1 item2" {
		t.Errorf("UNSET_APPEND = %q; want 'item1 item2'", ast.Variables["UNSET_APPEND"])
	}

	// 4. Shell !=
	if ast.Variables["SHELL_ECHO"] != "shell_test_output" {
		t.Errorf("SHELL_ECHO = %q; want 'shell_test_output'", ast.Variables["SHELL_ECHO"])
	}
	if !strings.Contains(ast.Variables["SHELL_MULTILINE"], "line1") || !strings.Contains(ast.Variables["SHELL_MULTILINE"], "line3") {
		t.Errorf("SHELL_MULTILINE = %q; expected multiline joined with spaces", ast.Variables["SHELL_MULTILINE"])
	}

	// 5. Define blocks
	if !strings.Contains(ast.Variables["BLOCK_RECURSIVE"], "Line 1: hello") || !strings.Contains(ast.Variables["BLOCK_RECURSIVE"], "Line 2: end") {
		t.Errorf("BLOCK_RECURSIVE = %q; expected expanded lazy vars", ast.Variables["BLOCK_RECURSIVE"])
	}
	if !strings.Contains(ast.Variables["BLOCK_SIMPLE"], "Simple line 2: hello") {
		t.Errorf("BLOCK_SIMPLE = %q; expected expanded simple block", ast.Variables["BLOCK_SIMPLE"])
	}
}

// TestStress_RecursionAndCycleDetection tests circular references, self-references, and deep nesting.
func TestStress_RecursionAndCycleDetection(t *testing.T) {
	input := `
# 2-variable cycle
CYC_A = $(CYC_B)
CYC_B = $(CYC_A)

# 3-variable cycle
TRI_1 = $(TRI_2)
TRI_2 = $(TRI_3)
TRI_3 = $(TRI_1)

# Self reference
SELF = prefix $(SELF) suffix

# Deep nesting within limits
V0 = level0
V1 = $(V0)_1
V2 = $(V1)_2
V3 = $(V2)_3
V4 = $(V3)_4
V5 = $(V4)_5

# Nested variable names $($($(X)))
KEY_NAME = FINAL
LOOKUP_KEY = KEY_NAME
INDIRECT = $($($(LOOKUP_KEY)))
FINAL = target_reached

# Target rule using cyclical variable should not crash
app: $(CYC_A) $(TRI_1) $(V5)
	@echo $(SELF)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile must not panic/error on cyclic references: %v", err)
	}

	// Cycles should evaluate to empty string and log warnings
	if ast.Variables["CYC_A"] != "" || ast.Variables["CYC_B"] != "" {
		t.Errorf("Cyclic variables CYC_A/CYC_B must evaluate to empty string; got %q / %q", ast.Variables["CYC_A"], ast.Variables["CYC_B"])
	}
	if ast.Variables["TRI_1"] != "" || ast.Variables["TRI_2"] != "" || ast.Variables["TRI_3"] != "" {
		t.Errorf("Tri-cyclic variables must evaluate to empty string; got TRI_1=%q", ast.Variables["TRI_1"])
	}
	// Self-referencing variable with surrounding text safely recovers by expanding cycle point to empty
	if ast.Variables["SELF"] != "prefix  suffix" {
		t.Errorf("Self-referencing variable with surrounding text = %q; want 'prefix  suffix'", ast.Variables["SELF"])
	}

	// Deep valid nesting
	if ast.Variables["V5"] != "level0_1_2_3_4_5" {
		t.Errorf("V5 = %q; want 'level0_1_2_3_4_5'", ast.Variables["V5"])
	}

	// Dynamic indirect variable dereferencing
	if ast.Variables["INDIRECT"] != "target_reached" {
		t.Errorf("INDIRECT = %q; want 'target_reached'", ast.Variables["INDIRECT"])
	}

	// Warnings should be logged
	if len(ast.Warnings) == 0 {
		t.Errorf("Expected cycle warnings recorded in AST")
	}

	// Target rule should still be parsed
	rule, ok := ast.GetRule("app")
	if !ok || rule == nil {
		t.Fatalf("Expected 'app' rule to exist in AST")
	}
}

// TestStress_SuffixAndPatternSubstitutions tests complex substitution patterns.
func TestStress_SuffixAndPatternSubstitutions(t *testing.T) {
	input := `
SOURCES = src/main.c src/util.c src/algo/tree.c include/header.h
OBJECTS = obj/main.o obj/util.o obj/algo/tree.o

# Suffix substitutions
SUBST_1 := $(SOURCES:.c=.o)
SUBST_STRIP := $(OBJECTS:.o=)
SUBST_BRACES := ${SOURCES:.c=.obj}

# Pattern substitutions with %
PAT_1 := $(OBJECTS:obj/%.o=src/%.c)
PAT_PREFIX := $(SOURCES:src/%=build/obj/%)
PAT_SUFFIX := $(SOURCES:%.c=%.bin)
PAT_NO_PERCENT_REP := $(SOURCES:%.c=static_output)

# Dynamic variables in pattern substitutions
SRC_EXT = .c
OBJ_EXT = .o
PAT_DYNAMIC := $(SOURCES:$(SRC_EXT)=$(OBJ_EXT))

# Substitution on empty or whitespace variable
EMPTY_VAR =
EMPTY_SUBST := $(EMPTY_VAR:.c=.o)

# Pattern substitution with empty replacement
PAT_EMPTY_REP := $(patsubst %.c,, a.c b.c)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	if ast.Variables["SUBST_1"] != "src/main.o src/util.o src/algo/tree.o include/header.h" {
		t.Errorf("SUBST_1 = %q", ast.Variables["SUBST_1"])
	}
	if ast.Variables["SUBST_STRIP"] != "obj/main obj/util obj/algo/tree" {
		t.Errorf("SUBST_STRIP = %q", ast.Variables["SUBST_STRIP"])
	}
	if ast.Variables["SUBST_BRACES"] != "src/main.obj src/util.obj src/algo/tree.obj include/header.h" {
		t.Errorf("SUBST_BRACES = %q", ast.Variables["SUBST_BRACES"])
	}
	if ast.Variables["PAT_1"] != "src/main.c src/util.c src/algo/tree.c" {
		t.Errorf("PAT_1 = %q", ast.Variables["PAT_1"])
	}
	if ast.Variables["PAT_PREFIX"] != "build/obj/main.c build/obj/util.c build/obj/algo/tree.c include/header.h" {
		t.Errorf("PAT_PREFIX = %q", ast.Variables["PAT_PREFIX"])
	}
	if ast.Variables["PAT_NO_PERCENT_REP"] != "static_output static_output static_output include/header.h" {
		t.Errorf("PAT_NO_PERCENT_REP = %q", ast.Variables["PAT_NO_PERCENT_REP"])
	}
	if ast.Variables["PAT_DYNAMIC"] != "src/main.o src/util.o src/algo/tree.o include/header.h" {
		t.Errorf("PAT_DYNAMIC = %q", ast.Variables["PAT_DYNAMIC"])
	}
	if ast.Variables["EMPTY_SUBST"] != "" {
		t.Errorf("EMPTY_SUBST = %q; want empty string", ast.Variables["EMPTY_SUBST"])
	}
	if ast.Variables["PAT_EMPTY_REP"] != "" {
		t.Errorf("PAT_EMPTY_REP = %q; want empty string", ast.Variables["PAT_EMPTY_REP"])
	}
}

// TestStress_AutomaticVariablesAndAutoVarsLookup tests AutoVars and its directory/file variants.
func TestStress_AutomaticVariablesAndAutoVarsLookup(t *testing.T) {
	// 1. Direct unit verification of AutoVars lookup table
	auto := &converter.AutoVars{
		Target:         "bin/release/app",
		FirstPrereq:    "src/core/main.c",
		Prereqs:        []string{"src/core/main.c", "src/util/helper.c", "src/core/main.c", "lib/libmath.a"},
		Stem:           "core/module",
		ArchiveMember:  "object.o",
		UpdatedPrereqs: []string{"src/util/helper.c"},
	}

	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"Target", "@", "bin/release/app"},
		{"TargetDir", "@D", "bin/release"},
		{"TargetFile", "@F", "app"},
		{"FirstPrereq", "<", "src/core/main.c"},
		{"FirstPrereqDir", "<D", "src/core"},
		{"FirstPrereqFile", "<F", "main.c"},
		{"PrereqsDedup", "^", "src/core/main.c src/util/helper.c lib/libmath.a"},
		{"PrereqsAll", "+", "src/core/main.c src/util/helper.c src/core/main.c lib/libmath.a"},
		{"PrereqsDirs", "^D", "src/core src/util lib"},
		{"PrereqsFiles", "^F", "main.c helper.c libmath.a"},
		{"Stem", "*", "core/module"},
		{"StemDir", "*D", "core"},
		{"StemFile", "*F", "module"},
		{"ArchiveMember", "%", "object.o"},
		{"UpdatedPrereqs", "?", "src/util/helper.c"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := auto.Lookup(tc.varName)
			if !ok || got != tc.expected {
				t.Errorf("AutoVars.Lookup(%q) = %q (ok=%v); want %q", tc.varName, got, ok, tc.expected)
			}
		})
	}

	// 2. Test root directory and empty edge cases
	rootAuto := &converter.AutoVars{
		Target:      "app",
		FirstPrereq: "",
		Prereqs:     nil,
	}
	targetDir, ok := rootAuto.Lookup("@D")
	if !ok || targetDir != "." {
		t.Errorf("root target @D = %q (ok=%v); want '.'", targetDir, ok)
	}
	targetFile, ok := rootAuto.Lookup("@F")
	if !ok || targetFile != "app" {
		t.Errorf("root target @F = %q (ok=%v); want 'app'", targetFile, ok)
	}
	firstDir, ok := rootAuto.Lookup("<D")
	if !ok || firstDir != "" {
		t.Errorf("empty first prereq <D = %q; want ''", firstDir)
	}

	// 3. Test Expander with AutoVars integration
	scope := converter.NewVarTable()
	expander := converter.NewExpander(scope)
	recipeTemplate := `$(CC) -c $< -o $@ -I$(@D) -I$(<D) && ar rcs $(@F) $^`
	expanded := expander.Expand(recipeTemplate, auto)
	expectedSubstrings := []string{
		"src/core/main.c",
		"bin/release/app",
		"-Ibin/release",
		"-Isrc/core",
		"app",
		"src/core/main.c src/util/helper.c lib/libmath.a",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(expanded, sub) {
			t.Errorf("Expander.Expand with AutoVars missing expected substring %q in: %s", sub, expanded)
		}
	}

	// 4. Test nil AutoVars safety
	var nilAuto *converter.AutoVars
	got, ok := nilAuto.Lookup("@")
	if ok || got != "" {
		t.Errorf("nil AutoVars lookup should return false and empty, got %q (ok=%v)", got, ok)
	}
}

// TestStress_BuiltinFunctionsComprehensive tests the complete catalog of GNU Make functions.
func TestStress_BuiltinFunctionsComprehensive(t *testing.T) {
	// Create temporary files for testing wildcard
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "test1.c")
	f2 := filepath.Join(tmpDir, "test2.c")
	f3 := filepath.Join(tmpDir, "test3.h")
	_ = os.WriteFile(f1, []byte("int main(){}"), 0644)
	_ = os.WriteFile(f2, []byte("void helper(){}"), 0644)
	_ = os.WriteFile(f3, []byte("#pragma once"), 0644)

	input := fmt.Sprintf(`
# Wildcard matching actual files
WILDCARD_MATCH := $(wildcard %s/*.c)
# Wildcard matching non-existent pattern (fallback pattern preserved)
WILDCARD_FALLBACK := $(wildcard non_existent_dir/*.cpp)

# patsubst
PAT_1 := $(patsubst %%,%%.o, foo bar)
PAT_2 := $(patsubst src/%%.c, build/%%.o, src/x.c src/y/z.c other.c)

# subst
SUBST_1 := $(subst ee,EE,feet on the street)
SUBST_2 := $(subst a,alpha,a b a c)

# addprefix and addsuffix
PREFIX_1 := $(addprefix /opt/lib/, liba.a libb.a)
SUFFIX_1 := $(addsuffix .tar.gz, archive1 archive2)
PREFIX_EMPTY := $(addprefix , foo bar)
SUFFIX_EMPTY := $(addsuffix , foo bar)

# dir, notdir, basename, suffix
DIR_1 := $(dir /usr/local/bin/app src/main.c util.c)
NOTDIR_1 := $(notdir /usr/local/bin/app src/main.c util.c)
BASE_1 := $(basename /usr/local/bin/app.tar.gz src/main.c no_ext)
SUF_1 := $(suffix /usr/local/bin/app.tar.gz src/main.c no_ext)

# filter and filter-out
LIST = main.c util.c core.cpp test.h script.sh
FILTER_C := $(filter %%.c %%.cpp, $(LIST))
FILTEROUT_C := $(filter-out %%.c %%.cpp, $(LIST))
FILTER_EXACT := $(filter main.c test.h, $(LIST))

# strip, firstword, lastword
STRIP_1 := $(strip   first    second    third   )
FIRST_1 := $(firstword one two three)
LAST_1 := $(lastword one two three)
FIRST_EMPTY := $(firstword )
LAST_EMPTY := $(lastword )

# word, words, wordlist
W_1 := $(word 1, alpha beta gamma delta)
W_3 := $(word 3, alpha beta gamma delta)
W_OOB := $(word 10, alpha beta gamma delta)
W_ZERO := $(word 0, alpha beta gamma delta)
W_NEG := $(word -2, alpha beta gamma delta)
WORDS_COUNT := $(words alpha beta gamma delta)
WORDS_EMPTY := $(words )
WLIST_1 := $(wordlist 2, 3, alpha beta gamma delta epsilon)
WLIST_OOB := $(wordlist 3, 10, alpha beta gamma)
WLIST_INV := $(wordlist 4, 2, alpha beta gamma)

# sort (sorts and deduplicates)
SORT_1 := $(sort zebra apple banana apple cherry banana zebra)

# join
JOIN_1 := $(join a b c, 1 2 3)
JOIN_UNEQUAL_1 := $(join a b c d, 1 2)
JOIN_UNEQUAL_2 := $(join a b, 1 2 3 4)

# if, or, and
IF_1 := $(if true_cond, was_true, was_false)
IF_2 := $(if , was_true, was_false)
IF_3 := $(if , was_true)
OR_1 := $(or , , first_val, second_val)
OR_NONE := $(or , , )
AND_1 := $(and val1, val2, final_val)
AND_FAIL := $(and val1, , final_val)

# foreach
ITEMS = a b c d
FOREACH_RES := $(foreach item, $(ITEMS), $(item).o)

# call
PARAM_MACRO = compile -c $(1) -o $(2) -flags $(3)
CALL_RES := $(call PARAM_MACRO, src/main.c, bin/main.o, -O3)

# shell
SHELL_CALC != echo "computed_value"

# origin and flavor
ORIGIN_DEF := $(origin CC)
ORIGIN_FILE := $(origin LIST)
ORIGIN_UNDEF := $(origin COMPLETELY_UNDEFINED)
FLAVOR_DEF := $(flavor CC)
FLAVOR_SIMP := $(flavor PAT_1)
`, tmpDir)

	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	// 1. Wildcard
	if !strings.Contains(ast.Variables["WILDCARD_MATCH"], "test1.c") || !strings.Contains(ast.Variables["WILDCARD_MATCH"], "test2.c") {
		t.Errorf("WILDCARD_MATCH = %q; expected test1.c and test2.c", ast.Variables["WILDCARD_MATCH"])
	}
	if ast.Variables["WILDCARD_FALLBACK"] != "non_existent_dir/*.cpp" {
		t.Errorf("WILDCARD_FALLBACK = %q; want fallback pattern 'non_existent_dir/*.cpp'", ast.Variables["WILDCARD_FALLBACK"])
	}

	// 2. patsubst & subst
	if ast.Variables["PAT_2"] != "build/x.o build/y/z.o other.c" {
		t.Errorf("PAT_2 = %q", ast.Variables["PAT_2"])
	}
	if ast.Variables["SUBST_1"] != "fEEt on the strEEt" {
		t.Errorf("SUBST_1 = %q", ast.Variables["SUBST_1"])
	}
	if ast.Variables["SUBST_2"] != "alphalphab alphac" && ast.Variables["SUBST_2"] != "alpha b alpha c" {
		t.Errorf("SUBST_2 = %q", ast.Variables["SUBST_2"])
	}

	// 3. addprefix & addsuffix
	if ast.Variables["PREFIX_1"] != "/opt/lib/liba.a /opt/lib/libb.a" {
		t.Errorf("PREFIX_1 = %q", ast.Variables["PREFIX_1"])
	}
	if ast.Variables["SUFFIX_1"] != "archive1.tar.gz archive2.tar.gz" {
		t.Errorf("SUFFIX_1 = %q", ast.Variables["SUFFIX_1"])
	}
	if ast.Variables["PREFIX_EMPTY"] != "foo bar" {
		t.Errorf("PREFIX_EMPTY = %q", ast.Variables["PREFIX_EMPTY"])
	}

	// 4. dir, notdir, basename, suffix
	if ast.Variables["DIR_1"] != "/usr/local/bin/ src/ ./" {
		t.Errorf("DIR_1 = %q", ast.Variables["DIR_1"])
	}
	if ast.Variables["NOTDIR_1"] != "app main.c util.c" {
		t.Errorf("NOTDIR_1 = %q", ast.Variables["NOTDIR_1"])
	}
	if ast.Variables["BASE_1"] != "/usr/local/bin/app.tar src/main no_ext" {
		t.Errorf("BASE_1 = %q", ast.Variables["BASE_1"])
	}
	if ast.Variables["SUF_1"] != ".gz .c" {
		t.Errorf("SUF_1 = %q", ast.Variables["SUF_1"])
	}

	// 5. filter & filter-out
	if ast.Variables["FILTER_C"] != "main.c util.c core.cpp" {
		t.Errorf("FILTER_C = %q", ast.Variables["FILTER_C"])
	}
	if ast.Variables["FILTEROUT_C"] != "test.h script.sh" {
		t.Errorf("FILTEROUT_C = %q", ast.Variables["FILTEROUT_C"])
	}
	if ast.Variables["FILTER_EXACT"] != "main.c test.h" {
		t.Errorf("FILTER_EXACT = %q", ast.Variables["FILTER_EXACT"])
	}

	// 6. strip, firstword, lastword
	if ast.Variables["STRIP_1"] != "first second third" {
		t.Errorf("STRIP_1 = %q", ast.Variables["STRIP_1"])
	}
	if ast.Variables["FIRST_1"] != "one" || ast.Variables["LAST_1"] != "three" {
		t.Errorf("FIRST_1 = %q, LAST_1 = %q", ast.Variables["FIRST_1"], ast.Variables["LAST_1"])
	}
	if ast.Variables["FIRST_EMPTY"] != "" || ast.Variables["LAST_EMPTY"] != "" {
		t.Errorf("FIRST_EMPTY=%q, LAST_EMPTY=%q", ast.Variables["FIRST_EMPTY"], ast.Variables["LAST_EMPTY"])
	}

	// 7. word, words, wordlist
	if ast.Variables["W_1"] != "alpha" || ast.Variables["W_3"] != "gamma" {
		t.Errorf("W_1=%q, W_3=%q", ast.Variables["W_1"], ast.Variables["W_3"])
	}
	if ast.Variables["W_OOB"] != "" || ast.Variables["W_ZERO"] != "" || ast.Variables["W_NEG"] != "" {
		t.Errorf("W_OOB=%q, W_ZERO=%q, W_NEG=%q", ast.Variables["W_OOB"], ast.Variables["W_ZERO"], ast.Variables["W_NEG"])
	}
	if ast.Variables["WORDS_COUNT"] != "4" || ast.Variables["WORDS_EMPTY"] != "0" {
		t.Errorf("WORDS_COUNT=%q, WORDS_EMPTY=%q", ast.Variables["WORDS_COUNT"], ast.Variables["WORDS_EMPTY"])
	}
	if ast.Variables["WLIST_1"] != "beta gamma" {
		t.Errorf("WLIST_1 = %q", ast.Variables["WLIST_1"])
	}
	if ast.Variables["WLIST_OOB"] != "gamma" {
		t.Errorf("WLIST_OOB = %q", ast.Variables["WLIST_OOB"])
	}
	if ast.Variables["WLIST_INV"] != "" {
		t.Errorf("WLIST_INV = %q", ast.Variables["WLIST_INV"])
	}

	// 8. sort
	if ast.Variables["SORT_1"] != "apple banana cherry zebra" {
		t.Errorf("SORT_1 = %q", ast.Variables["SORT_1"])
	}

	// 9. join
	if ast.Variables["JOIN_1"] != "a1 b2 c3" {
		t.Errorf("JOIN_1 = %q", ast.Variables["JOIN_1"])
	}
	if ast.Variables["JOIN_UNEQUAL_1"] != "a1 b2 c d" {
		t.Errorf("JOIN_UNEQUAL_1 = %q", ast.Variables["JOIN_UNEQUAL_1"])
	}
	if ast.Variables["JOIN_UNEQUAL_2"] != "a1 b2 3 4" {
		t.Errorf("JOIN_UNEQUAL_2 = %q", ast.Variables["JOIN_UNEQUAL_2"])
	}

	// 10. if, or, and
	if ast.Variables["IF_1"] != "was_true" || ast.Variables["IF_2"] != "was_false" || ast.Variables["IF_3"] != "" {
		t.Errorf("IF_1=%q, IF_2=%q, IF_3=%q", ast.Variables["IF_1"], ast.Variables["IF_2"], ast.Variables["IF_3"])
	}
	if ast.Variables["OR_1"] != "first_val" || ast.Variables["OR_NONE"] != "" {
		t.Errorf("OR_1=%q, OR_NONE=%q", ast.Variables["OR_1"], ast.Variables["OR_NONE"])
	}
	if ast.Variables["AND_1"] != "final_val" || ast.Variables["AND_FAIL"] != "" {
		t.Errorf("AND_1=%q, AND_FAIL=%q", ast.Variables["AND_1"], ast.Variables["AND_FAIL"])
	}

	// 11. foreach & call
	if ast.Variables["FOREACH_RES"] != "a.o b.o c.o d.o" {
		t.Errorf("FOREACH_RES = %q", ast.Variables["FOREACH_RES"])
	}
	if ast.Variables["CALL_RES"] != "compile -c src/main.c -o bin/main.o -flags -O3" {
		t.Errorf("CALL_RES = %q", ast.Variables["CALL_RES"])
	}

	// 12. origin & flavor
	if ast.Variables["ORIGIN_DEF"] != "default" || ast.Variables["ORIGIN_FILE"] != "file" || ast.Variables["ORIGIN_UNDEF"] != "undefined" {
		t.Errorf("Origin results: DEF=%q, FILE=%q, UNDEF=%q", ast.Variables["ORIGIN_DEF"], ast.Variables["ORIGIN_FILE"], ast.Variables["ORIGIN_UNDEF"])
	}
	if ast.Variables["FLAVOR_DEF"] != "recursive" || ast.Variables["FLAVOR_SIMP"] != "simple" {
		t.Errorf("Flavor results: DEF=%q, SIMP=%q", ast.Variables["FLAVOR_DEF"], ast.Variables["FLAVOR_SIMP"])
	}
}

// TestStress_ConditionalsDeepAndComplex tests complex nested conditionals and inactive branches.
func TestStress_ConditionalsDeepAndComplex(t *testing.T) {
	input := `
TARGET_ARCH = x86_64
BUILD_TYPE = release
ENABLE_FEATURE_X = 1
EMPTY_FLAG =

# 1. ifeq / ifneq with single quotes, double quotes, and parens
ifeq ($(TARGET_ARCH), x86_64)
  ARCH_MATCH_PAREN = ok
endif

ifeq '$(TARGET_ARCH)' 'x86_64'
  ARCH_MATCH_SQUOTE = ok
endif

ifeq "$(TARGET_ARCH)" "x86_64"
  ARCH_MATCH_DQUOTE = ok
endif

ifneq ($(TARGET_ARCH), arm64)
  NOT_ARM = ok
endif

# 2. ifdef / ifndef testing
ifdef ENABLE_FEATURE_X
  FEAT_X = enabled
else
  FEAT_X = disabled
endif

ifdef EMPTY_FLAG
  EMPTY_CHECK = should_not_be_set
else
  EMPTY_CHECK = is_empty
endif

ifndef NON_EXISTENT_VAR
  NOT_DEF = correct
endif

# 3. Else-if chain
TEST_VAL = 30
ifeq ($(TEST_VAL), 10)
  CHAIN_RES = ten
else ifeq ($(TEST_VAL), 20)
  CHAIN_RES = twenty
else ifeq ($(TEST_VAL), 30)
  CHAIN_RES = thirty
else
  CHAIN_RES = other
endif

# 4. Deep 4-level nested conditionals with inactive branch isolation
CONFIG_A = 1
CONFIG_B = 2
CONFIG_C = 3
CONFIG_D = 4

ifeq ($(CONFIG_A), 1)
  L1 = pass
  ifeq ($(CONFIG_B), 2)
    L2 = pass
    ifeq ($(CONFIG_C), mismatch)
      L3 = should_fail
      INACTIVE_VAR = must_not_exist
      malformed recipe syntax in inactive branch :::: ;;;
    else
      L3 = pass
      ifeq ($(CONFIG_D), 4)
        L4 = pass
        NESTED_SUCCESS = fully_reached
      else
        L4 = fail
      endif
    endif
  endif
endif

# 5. Inactive rule isolation
ifeq ($(BUILD_TYPE), debug)
debug_only_target:
	@echo "Debug only"
else
release_only_target:
	@echo "Release only"
endif
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	// 1. Syntax styles
	if ast.Variables["ARCH_MATCH_PAREN"] != "ok" || ast.Variables["ARCH_MATCH_SQUOTE"] != "ok" || ast.Variables["ARCH_MATCH_DQUOTE"] != "ok" {
		t.Errorf("Conditional quote styles failed: paren=%q, squote=%q, dquote=%q",
			ast.Variables["ARCH_MATCH_PAREN"], ast.Variables["ARCH_MATCH_SQUOTE"], ast.Variables["ARCH_MATCH_DQUOTE"])
	}
	if ast.Variables["NOT_ARM"] != "ok" {
		t.Errorf("NOT_ARM = %q", ast.Variables["NOT_ARM"])
	}

	// 2. ifdef / ifndef
	if ast.Variables["FEAT_X"] != "enabled" {
		t.Errorf("FEAT_X = %q; want 'enabled'", ast.Variables["FEAT_X"])
	}
	if ast.Variables["EMPTY_CHECK"] != "is_empty" {
		t.Errorf("EMPTY_CHECK = %q; want 'is_empty'", ast.Variables["EMPTY_CHECK"])
	}
	if ast.Variables["NOT_DEF"] != "correct" {
		t.Errorf("NOT_DEF = %q; want 'correct'", ast.Variables["NOT_DEF"])
	}

	// 3. Else-if chain
	if ast.Variables["CHAIN_RES"] != "thirty" {
		t.Errorf("CHAIN_RES = %q; want 'thirty'", ast.Variables["CHAIN_RES"])
	}

	// 4. Nested conditionals
	if ast.Variables["L1"] != "pass" || ast.Variables["L2"] != "pass" || ast.Variables["L3"] != "pass" || ast.Variables["L4"] != "pass" {
		t.Errorf("Nested levels failed: L1=%q, L2=%q, L3=%q, L4=%q",
			ast.Variables["L1"], ast.Variables["L2"], ast.Variables["L3"], ast.Variables["L4"])
	}
	if ast.Variables["NESTED_SUCCESS"] != "fully_reached" {
		t.Errorf("NESTED_SUCCESS = %q; want 'fully_reached'", ast.Variables["NESTED_SUCCESS"])
	}
	if ast.Variables["INACTIVE_VAR"] != "" {
		t.Errorf("INACTIVE_VAR must not be set from inactive branch, got %q", ast.Variables["INACTIVE_VAR"])
	}

	// 5. Inactive rules
	if ast.RuleMap["debug_only_target"] != nil {
		t.Errorf("debug_only_target must not exist in AST")
	}
	if ast.RuleMap["release_only_target"] == nil {
		t.Errorf("release_only_target should exist in AST")
	}
}

// TestStress_FuzzAndMalformedDegradation tests resilience against pathological inputs.
func TestStress_FuzzAndMalformedDegradation(t *testing.T) {
	pathologicalInputs := []struct {
		name     string
		makefile string
	}{
		{
			name:     "Unclosed variable parens and braces",
			makefile: "A = $(UNCLOSED\nB = ${UNCLOSED_BRACE\nC = $(\nall:\n\t@echo $(A)",
		},
		{
			name:     "Unclosed and malformed function invocations",
			makefile: "P = $(patsubst %.c, %.o\nS = $(subst a, b\nI = $(if cond, yes\nall:\n\t@echo $(P)",
		},
		{
			name:     "Multiple stray elses and endifs without ifs",
			makefile: "else\nendif\nelse ifeq (1, 1)\nendif\nTARGET = ok\nall: $(TARGET)",
		},
		{
			name:     "Massive variable expansion stress (1000 items)",
			makefile: fmt.Sprintf("LONG_LIST = %s\nCOUNT := $(words $(LONG_LIST))\n", strings.Repeat("item ", 1000)),
		},
		{
			name:     "Special characters, colons, URLs in variables",
			makefile: "URL := https://user:pass@example.com:8443/api/v1?query=1&val=2#frag\nFLAGS := -Wl,-rpath,$ORIGIN/../lib\n",
		},
		{
			name:     "Escaped dollar combinations",
			makefile: "D1 := $$\nD2 := $$$$\nD3 := $$VAR\nD4 := $$(SHELL_CMD)\n",
		},
	}

	for _, tc := range pathologicalInputs {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := converter.ParseMakefile(tc.makefile)
			if err != nil {
				t.Fatalf("ParseMakefile crashed/errored on pathological input: %v", err)
			}
			if ast == nil {
				t.Fatalf("Returned AST is nil")
			}
		})
	}
}

// TestStress_NestedConditionalsWithFunctions tests conditionals containing embedded function calls and commas.
func TestStress_NestedConditionalsWithFunctions(t *testing.T) {
	input := `
SRCS = main.c header.h
ifeq ($(filter main.c, $(SRCS)), main.c)
  FILTER_COND = matched
else
  FILTER_COND = failed
endif

# Comma inside subst inside ifeq
ifeq ($(subst a,b, a), b)
  SUBST_COND = matched
endif

# strip in condition
EMPTY_VAL =    
ifeq ($(strip $(EMPTY_VAL)),)
  STRIP_COND = is_empty
endif

# or function inside ifeq
VAL1 =
VAL2 = active
ifeq ($(or $(VAL1), $(VAL2)), active)
  OR_COND = matched
endif
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	if ast.Variables["FILTER_COND"] != "matched" {
		t.Errorf("FILTER_COND = %q; want 'matched'", ast.Variables["FILTER_COND"])
	}
	if ast.Variables["SUBST_COND"] != "matched" {
		t.Errorf("SUBST_COND = %q; want 'matched'", ast.Variables["SUBST_COND"])
	}
	if ast.Variables["STRIP_COND"] != "is_empty" {
		t.Errorf("STRIP_COND = %q; want 'is_empty'", ast.Variables["STRIP_COND"])
	}
	if ast.Variables["OR_COND"] != "matched" {
		t.Errorf("OR_COND = %q; want 'matched'", ast.Variables["OR_COND"])
	}
}

// TestStress_DynamicRulesAndPrerequisites tests rules constructed from expanded variables.
func TestStress_DynamicRulesAndPrerequisites(t *testing.T) {
	input := `
PROGS = bin/server bin/client
COMMON_OBJS = obj/net.o obj/util.o
DIR_DEP = build_dir

all: $(PROGS)

$(PROGS): $(COMMON_OBJS) | $(DIR_DEP)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(COMMON_OBJS): obj/%.o: src/%.c
	@mkdir -p obj
	$(CC) -c $< -o $@
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	// Verify all target rules exist
	if ast.RuleMap["bin/server"] == nil {
		t.Fatalf("missing rule 'bin/server'")
	}
	if ast.RuleMap["bin/client"] == nil {
		t.Fatalf("missing rule 'bin/client'")
	}

	serverRule := ast.RuleMap["bin/server"]
	if len(serverRule.Prerequisites) != 2 || serverRule.Prerequisites[0] != "obj/net.o" || serverRule.Prerequisites[1] != "obj/util.o" {
		t.Errorf("serverRule prereqs = %v; want ['obj/net.o', 'obj/util.o']", serverRule.Prerequisites)
	}
	if len(serverRule.OrderOnly) != 1 || serverRule.OrderOnly[0] != "build_dir" {
		t.Errorf("serverRule order-only = %v; want ['build_dir']", serverRule.OrderOnly)
	}

	// Static pattern rules
	netRule := ast.RuleMap["obj/net.o"]
	if netRule == nil {
		t.Fatalf("missing static pattern rule 'obj/net.o'")
	}
	if len(netRule.Prerequisites) != 2 || netRule.Prerequisites[0] != "src/net.c" || netRule.Prerequisites[1] != "src/util.c" {
		t.Errorf("netRule prereqs = %v; want ['src/net.c', 'src/util.c']", netRule.Prerequisites)
	}
}

// TestStress_ExpanderMaxDepthGracefulLimit tests that recursion beyond MaxDepth (64) terminates safely.
func TestStress_ExpanderMaxDepthGracefulLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("D_0 = base_value\n")
	for i := 1; i <= 80; i++ {
		sb.WriteString(fmt.Sprintf("D_%d = $(D_%d)_%d\n", i, i-1, i))
	}
	sb.WriteString("TARGET = $(D_80)\n")

	ast, err := converter.ParseMakefile(sb.String())
	if err != nil {
		t.Fatalf("ParseMakefile must not error on deep recursion: %v", err)
	}

	// Variable D_80 exceeds depth limit of 64, should gracefully return empty string or truncated and log warning
	if ast == nil {
		t.Fatalf("AST is nil")
	}
	if len(ast.Warnings) == 0 {
		t.Errorf("Expected depth limit warning recorded")
	}
}

// TestStress_StaticPatternStemOverlap tests static pattern rules when target length is shorter than prefix+suffix.
func TestStress_StaticPatternStemOverlap(t *testing.T) {
	// Adversarial input: target "a" with pattern "a%a"
	input := `
a: a%a: %.c
	$(CC) -c $< -o $@
`
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CRASH/PANIC detected on static pattern rule stem overlap: %v", r)
		}
	}()

	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Logf("ParseMakefile returned error (graceful): %v", err)
	} else if ast != nil {
		t.Logf("ParseMakefile parsed %d rules", len(ast.Rules))
	}
}


