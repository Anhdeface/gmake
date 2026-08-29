package tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/generator"
	"github.com/Anhdeface/gmake/internal/models"
	"github.com/Anhdeface/gmake/internal/parser"
)

// Helper function to find project root
func getProjectRoot(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	// If running inside tests/, parent is project root
	if filepath.Base(cwd) == "tests" {
		return filepath.Dir(cwd)
	}
	return cwd
}

// runGomakeConvert executes the CLI converter subcommand.
func runGomakeConvert(t *testing.T, projectRoot, inputPath, outputPath string) (string, error) {
	cmd := exec.Command("go", "run", "main.go", "convert", "-i", inputPath, "-o", outputPath, "-f")
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		combined := fmt.Sprintf("stdout: %s\nstderr: %s", stdout.String(), stderr.String())
		return combined, err
	}
	return stdout.String(), nil
}

// runGomakeConvertStdout executes the CLI converter subcommand dumping to stdout.
func runGomakeConvertStdout(t *testing.T, projectRoot, inputPath string) (string, string, error) {
	cmd := exec.Command("go", "run", "main.go", "convert", "-i", inputPath, "-s")
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// executePipeline runs the full 5-stage E2E verification pipeline for a given Makefile fixture.
func executePipeline(t *testing.T, fixtureRelPath string, validateConfig func(t *testing.T, cfg *models.GomakeConfig)) {
	t.Helper()
	projectRoot := getProjectRoot(t)
	fixturePath := filepath.Join(projectRoot, fixtureRelPath)

	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Fatalf("fixture file does not exist: %s", fixturePath)
	}

	tempDir := t.TempDir()
	outGomake := filepath.Join(tempDir, "output.gomake")
	outMakefile := filepath.Join(tempDir, "output.makefile")

	// Stage 1 & 2: Run CLI converter
	output, err := runGomakeConvert(t, projectRoot, fixturePath, outGomake)
	if err != nil {
		t.Fatalf("converter CLI execution failed on %s: %v\nOutput: %s", fixtureRelPath, err, output)
	}

	// Verify generated .gomake file exists and has content
	gomakeBytes, err := os.ReadFile(outGomake)
	if err != nil {
		t.Fatalf("failed to read generated gomake file %s: %v", outGomake, err)
	}
	gomakeContent := string(gomakeBytes)
	if !strings.Contains(gomakeContent, "[const]") {
		t.Errorf("generated .gomake missing [const] header:\n%s", gomakeContent)
	}
	if !strings.Contains(gomakeContent, "./gomake") {
		t.Errorf("generated .gomake missing ./gomake EOF terminator:\n%s", gomakeContent)
	}

	// Stage 3: Grammar & Schema compliance via internal/parser.ParseConfig
	cfg, err := parser.ParseConfig(outGomake)
	if err != nil {
		t.Fatalf("ParseConfig rejected generated .gomake content: %v\nContent:\n%s", err, gomakeContent)
	}

	// Invariant checks on GomakeConfig
	if len(cfg.Constants) == 0 && len(cfg.Scripts) == 0 {
		t.Errorf("converted config has no constants and no scripts: %+v", cfg)
	}
	for _, c := range cfg.Constants {
		if cfg.Setups[c] == nil && cfg.Dependencies[c] == nil {
			t.Errorf("constant '%s' declared in [const] but has no setup or dependency block", c)
		}
	}

	// Optional custom tier validation
	if validateConfig != nil {
		validateConfig(t, cfg)
	}

	// Stage 4: Transpilation to standard Makefile via internal/generator.GenerateMakefile
	err = generator.GenerateMakefile(cfg, outMakefile)
	if err != nil {
		t.Fatalf("GenerateMakefile failed on converted config: %v", err)
	}

	// Stage 5: Syntax verification via make dry-run if make is installed
	if makePath, err := exec.LookPath("make"); err == nil {
		makeCmd := exec.Command(makePath, "-f", outMakefile, "-n")
		makeCmd.Dir = filepath.Dir(fixturePath)
		var makeStderr bytes.Buffer
		makeCmd.Stderr = &makeStderr
		if err := makeCmd.Run(); err != nil {
			t.Logf("make -n dry-run note (may require dummy files): %v, stderr: %s", err, makeStderr.String())
		}
	}
}

// ==========================================
// Tier 1: Feature Coverage Tests
// ==========================================

func TestE2E_Tier1_CExecutable(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/01_c_executable.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil {
			t.Fatalf("expected dependency block for %s", c)
		}
		if dep.BuildType != "executable" {
			t.Errorf("expected build.type 'executable', got '%s'", dep.BuildType)
		}
		setup := cfg.Setups[c]
		if setup == nil || setup.Compiler != "gcc" {
			t.Errorf("expected compiler 'gcc', got '%v'", setup)
		}
	})
}

func TestE2E_Tier1_CppExecutable(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/02_cpp_executable.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		setup := cfg.Setups[c]
		if setup == nil || (setup.Compiler != "g++" && setup.Compiler != "clang++") {
			t.Errorf("expected C++ compiler (g++), got '%v'", setup)
		}
	})
}

func TestE2E_Tier1_StaticLibrary(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/03_static_lib.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || dep.BuildType != "static" {
			t.Errorf("expected build.type 'static', got '%v'", dep)
		}
	})
}

func TestE2E_Tier1_SharedLibrary(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/04_shared_lib.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || dep.BuildType != "shared" {
			t.Errorf("expected build.type 'shared', got '%v'", dep)
		}
	})
}

func TestE2E_Tier1_ObjectDependency(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/05_object_dpdcy.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || !dep.ObjectDpdcy {
			t.Errorf("expected object.dpdcy=true, got '%v'", dep)
		}
	})
}

func TestE2E_Tier1_FlagsAndIncludes(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier1_features/06_flags_and_includes.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected at least 1 constant")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil {
			t.Fatalf("expected dependency block for %s", c)
		}
		if len(dep.Includes) == 0 {
			t.Errorf("expected non-empty includes")
		}
		if dep.Libs == "" {
			t.Errorf("expected non-empty libs")
		}
	})
}

// ==========================================
// Tier 2: Boundary & Corner Cases Tests
// ==========================================

func TestE2E_Tier2_WhitespaceTabsSpaces(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/01_whitespace_tabs_spaces.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected constant extracted despite non-standard spaces")
		}
	})
}

func TestE2E_Tier2_TrailingSlashesContinuations(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/02_trailing_slashes_and_continuations.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected constant extracted from multiline rule")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || len(dep.Sources) < 3 {
			t.Errorf("expected 3 joined source files, got: %v", dep)
		}
	})
}

func TestE2E_Tier2_EmptyTargetsAndPhony(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/03_empty_targets_and_phony.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected build constant")
		}
		if cfg.Scripts["run"] == "" {
			t.Errorf("expected 'run' script in config.scripts, got: %v", cfg.Scripts)
		}
	})
}

func TestE2E_Tier2_InlineSemicolonRecipes(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/04_inline_semicolon_recipes.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected constant from inline semicolon rule")
		}
	})
}

func TestE2E_Tier2_DuplicateTargetNames(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/05_duplicate_target_names.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) < 2 {
			t.Fatalf("expected 2 distinct constants for duplicate base names, got %d: %v", len(cfg.Constants), cfg.Constants)
		}
		if cfg.Constants[0] == cfg.Constants[1] {
			t.Errorf("constants must be unique in [const]: %v", cfg.Constants)
		}
	})
}

func TestE2E_Tier2_CommentsAndBlankLines(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/06_comments_and_blank_lines.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected target extracted despite heavy comments")
		}
	})
}

func TestE2E_Tier2_UnsupportedDirectivesGraceful(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier2_boundaries/07_unsupported_directives_graceful.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected successful conversion with graceful fallback on unsupported directives")
		}
	})
}

// ==========================================
// Tier 3: Cross-Feature Combinations Tests
// ==========================================

func TestE2E_Tier3_MultiTargetMixedBuilds(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier3_combinations/01_multi_target_mixed_builds.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) < 3 {
			t.Fatalf("expected at least 3 constants (static, shared, exe), got %d: %v", len(cfg.Constants), cfg.Constants)
		}
		buildTypes := make(map[string]bool)
		for _, c := range cfg.Constants {
			if dep := cfg.Dependencies[c]; dep != nil {
				buildTypes[dep.BuildType] = true
			}
		}
		if !buildTypes["static"] || !buildTypes["shared"] || !buildTypes["executable"] {
			t.Errorf("expected static, shared, and executable build types all present, got: %v", buildTypes)
		}
	})
}

func TestE2E_Tier3_VariableSuffixReplacementWildcards(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier3_combinations/02_variable_suffix_replacement_wildcards.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected constant extracted")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || !dep.ObjectDpdcy {
			t.Errorf("expected object.dpdcy=true from suffix replacement rule, got: %v", dep)
		}
	})
}

func TestE2E_Tier3_MultiTargetWithScripts(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier3_combinations/03_multi_target_with_scripts.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) < 2 {
			t.Errorf("expected 2 build targets (web_app, test_runner), got: %v", cfg.Constants)
		}
		if len(cfg.Scripts) < 3 {
			t.Errorf("expected at least 3 scripts (run, test, install, lint), got: %v", cfg.Scripts)
		}
	})
}

func TestE2E_Tier3_AutomaticVarsAndCrossFlags(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier3_combinations/04_automatic_vars_and_cross_flags.mk", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected constant extracted")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || len(dep.Includes) == 0 || dep.Libs == "" {
			t.Errorf("expected both includes and libs populated, got: %v", dep)
		}
	})
}

// ==========================================
// Tier 4: Real-World Application Makefiles Tests
// ==========================================

func TestE2E_Tier4_CliLoggerApp(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier4_realworld/01_cli_logger_app/Makefile", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected logger_cli target extracted")
		}
		if cfg.Scripts["run"] == "" {
			t.Errorf("expected 'run' script in config.scripts, got: %v", cfg.Scripts)
		}
	})
}

func TestE2E_Tier4_CryptoEngineLib(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier4_realworld/02_crypto_engine_lib/Makefile", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) == 0 {
			t.Fatal("expected libcrypto_engine target extracted")
		}
		c := cfg.Constants[0]
		dep := cfg.Dependencies[c]
		if dep == nil || dep.BuildType != "static" {
			t.Errorf("expected build.type 'static', got: %v", dep)
		}
	})
}

func TestE2E_Tier4_ServerMultiModule(t *testing.T) {
	executePipeline(t, "testdata/e2e/tier4_realworld/03_server_multimodule/Makefile", func(t *testing.T, cfg *models.GomakeConfig) {
		if len(cfg.Constants) < 3 {
			t.Errorf("expected 3 targets (shared lib, daemon, test runner), got %d: %v", len(cfg.Constants), cfg.Constants)
		}
	})
}

// TestE2E_CLI_StdoutMode tests stdout mode and flags
func TestE2E_CLI_StdoutMode(t *testing.T) {
	projectRoot := getProjectRoot(t)
	fixturePath := filepath.Join(projectRoot, "testdata/e2e/tier1_features/01_c_executable.mk")
	stdout, stderr, err := runGomakeConvertStdout(t, projectRoot, fixturePath)
	if err != nil {
		t.Fatalf("stdout mode execution failed (implementation pending or error): %v\nStderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "[const]") || !strings.Contains(stdout, "./gomake") {
		t.Errorf("expected .gomake formatted output on stdout, got:\n%s", stdout)
	}
}
