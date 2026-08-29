package converter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
	"github.com/Anhdeface/gmake/internal/models"
	"github.com/Anhdeface/gmake/internal/parser"
)

func TestSerializer_RoundTrip(t *testing.T) {
	cfg := &models.GomakeConfig{
		Constants: []string{"app", "lib_core"},
		Setups: map[string]*models.ConfigSetup{
			"app": {
				Compiler: "gcc",
				Flags:    "-Wall -O2",
				Name:     "my_app",
			},
			"lib_core": {
				Compiler: "gcc",
				Flags:    "-Wall -O2 -fPIC",
				Name:     "core",
			},
		},
		Dependencies: map[string]*models.ConfigDependency{
			"app": {
				Target:      "bin/my_app",
				Sources:     []string{"src/main.c", "src/util.c"},
				Includes:    []string{"include"},
				ObjectDpdcy: true,
				BuildType:   "executable",
				Libs:        "-Llib -lcore",
			},
			"lib_core": {
				Target:      "lib/libcore.a",
				Sources:     []string{"src/core.c"},
				Includes:    []string{"include"},
				ObjectDpdcy: false,
				BuildType:   "static",
				Libs:        "",
			},
		},
		Scripts: map[string]string{
			"run":  "./bin/my_app",
			"lint": "cppcheck src/",
		},
	}

	formatted, err := converter.FormatConfig(cfg)
	if err != nil {
		t.Fatalf("FormatConfig failed: %v", err)
	}

	// Verify required syntactic tokens
	if !strings.Contains(formatted, "[const]") {
		t.Errorf("missing [const]")
	}
	if !strings.Contains(formatted, "./gomake") {
		t.Errorf("missing ./gomake")
	}

	// Write to temp file and parse with parser.ParseConfig
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.gomake")
	if err := os.WriteFile(tmpFile, []byte(formatted), 0644); err != nil {
		t.Fatalf("failed to write tmp file: %v", err)
	}

	parsedCfg, err := parser.ParseConfig(tmpFile)
	if err != nil {
		t.Fatalf("ParseConfig failed to parse serialized output: %v\nFormatted Content:\n%s", err, formatted)
	}

	// Verify semantic equality
	if len(parsedCfg.Constants) != len(cfg.Constants) {
		t.Fatalf("constants length mismatch: %v vs %v", parsedCfg.Constants, cfg.Constants)
	}
	for i, c := range cfg.Constants {
		if parsedCfg.Constants[i] != c {
			t.Errorf("constant mismatch at %d: %s vs %s", i, parsedCfg.Constants[i], c)
		}
		if parsedCfg.Setups[c].Compiler != cfg.Setups[c].Compiler {
			t.Errorf("compiler mismatch for %s", c)
		}
		if parsedCfg.Dependencies[c].BuildType != cfg.Dependencies[c].BuildType {
			t.Errorf("build type mismatch for %s", c)
		}
	}
	if parsedCfg.Scripts["run"] != cfg.Scripts["run"] {
		t.Errorf("script mismatch: %s vs %s", parsedCfg.Scripts["run"], cfg.Scripts["run"])
	}
}

func TestSerializer_NilConfig(t *testing.T) {
	_, err := converter.FormatConfig(nil)
	if err == nil {
		t.Errorf("expected error when formatting nil config")
	}
}
