package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anhdeface/gmake/internal/models"
)

func TestGenerateMakefileUsesDistinctObjectsForCAndCppSources(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Makefile")
	config := &models.GomakeConfig{
		Dependency: models.ConfigDependency{
			Sources:     []string{"src/main.c", "src/main.cpp"},
			ObjectDpdcy: true,
		},
	}

	if err := GenerateMakefile(config, outputPath); err != nil {
		t.Fatalf("GenerateMakefile returned an error: %v", err)
	}

	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}

	for _, want := range []string{
		"OBJS = $(addsuffix .o, $(SRCS))",
		"%.c.o: %.c",
		"%.cpp.o: %.cpp",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Makefile does not contain %q", want)
		}
	}
}
