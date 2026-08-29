package converter_test

import (
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

func TestMakefileAST_Operations(t *testing.T) {
	ast := converter.NewMakefileAST()

	// 1. Variable operations
	ast.AddVariable(converter.VariableAssignment{
		Name:       "CC",
		Value:      "clang",
		RawValue:   "clang",
		Flavor:     converter.FlavorSimple,
		LineNumber: 1,
	})

	val, ok := ast.GetVariable("CC")
	if !ok || val != "clang" {
		t.Errorf("GetVariable('CC') = %q, %v; want 'clang', true", val, ok)
	}

	_, ok = ast.GetVariable("NONEXISTENT")
	if ok {
		t.Errorf("GetVariable('NONEXISTENT') should return false")
	}

	// 2. Phony targets registration
	ast.MarkPhony("clean", "test", "all")
	if !ast.PhonyTargets["clean"] || !ast.PhonyTargets["test"] || !ast.PhonyTargets["all"] {
		t.Errorf("MarkPhony failed to register phony targets: %v", ast.PhonyTargets)
	}

	// 3. Rule addition & DefaultGoal tracking
	patternRule := &converter.MakefileRule{
		Targets:       []string{"%.o"},
		Prerequisites: []string{"%.c"},
		IsPattern:     true,
		LineNumber:    5,
	}
	ast.AddRule(patternRule)
	if ast.DefaultGoal != "" {
		t.Errorf("pattern rule should not set DefaultGoal; got %q", ast.DefaultGoal)
	}

	dotRule := &converter.MakefileRule{
		Targets:    []string{".PHONY"},
		LineNumber: 10,
	}
	ast.AddRule(dotRule)
	if ast.DefaultGoal != "" {
		t.Errorf("dot rule should not set DefaultGoal; got %q", ast.DefaultGoal)
	}

	appRule := &converter.MakefileRule{
		Targets:       []string{"app", "bin/app"},
		Prerequisites: []string{"main.o"},
		Recipes:       []string{"gcc -o app main.o"},
		LineNumber:    15,
	}
	ast.AddRule(appRule)
	if ast.DefaultGoal != "app" {
		t.Errorf("expected DefaultGoal 'app', got %q", ast.DefaultGoal)
	}
	if appRule.Target() != "app" {
		t.Errorf("expected appRule.Target() = 'app', got %q", appRule.Target())
	}

	// RuleMap lookups
	r1, ok1 := ast.GetRule("app")
	if !ok1 || r1 != appRule {
		t.Errorf("GetRule('app') failed: %+v", r1)
	}
	r2, ok2 := ast.GetRule("bin/app")
	if !ok2 || r2 != appRule {
		t.Errorf("GetRule('bin/app') failed: %+v", r2)
	}

	// 4. MarkPhony after rule added
	ast.MarkPhony("app")
	if !appRule.IsPhony {
		t.Errorf("expected appRule.IsPhony to be true after MarkPhony")
	}

	// 5. Diagnostics
	ast.AddDiagnostic(20, "test warning", converter.SeverityWarn)
	if len(ast.Diagnostics) != 1 || ast.Diagnostics[0].Message != "test warning" {
		t.Errorf("Diagnostics error: %+v", ast.Diagnostics)
	}
	if len(ast.Warnings) != 1 || ast.Warnings[0] != "test warning" {
		t.Errorf("Warnings error: %+v", ast.Warnings)
	}
}
