package converter_test

import (
	"testing"

	"github.com/Anhdeface/gmake/internal/converter"
)

func TestConverter_CExecutable(t *testing.T) {
	input := `
CC = gcc
CFLAGS = -Wall -O2
SRCS = src/main.c src/util.c
TARGET = bin/my_c_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) -o $@ $(SRCS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	if len(cfg.Constants) != 1 || cfg.Constants[0] != "my_c_app" {
		t.Fatalf("expected constant 'my_c_app', got: %v", cfg.Constants)
	}

	setup := cfg.Setups["my_c_app"]
	if setup == nil || setup.Compiler != "gcc" || setup.Flags != "-Wall -O2" {
		t.Errorf("unexpected setup: %+v", setup)
	}

	dep := cfg.Dependencies["my_c_app"]
	if dep == nil || dep.Target != "bin/my_c_app" || dep.BuildType != "executable" || len(dep.Sources) != 2 {
		t.Errorf("unexpected dependency: %+v", dep)
	}
}

func TestConverter_CppExecutable(t *testing.T) {
	input := `
CXX = g++
CXXFLAGS = -std=c++17 -Wall -O3
SRCS = src/main.cpp src/solver.cpp
TARGET = bin/solver_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CXX) $(CXXFLAGS) -o $@ $(SRCS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	if len(cfg.Constants) != 1 {
		t.Fatalf("expected 1 constant, got: %v", cfg.Constants)
	}
	c := cfg.Constants[0]
	setup := cfg.Setups[c]
	if setup.Compiler != "g++" {
		t.Errorf("expected compiler 'g++', got: %s", setup.Compiler)
	}
}

func TestConverter_StaticLibrary(t *testing.T) {
	input := `
CC = gcc
CFLAGS = -Wall -O2
SRCS = src/math.c src/matrix.c
TARGET = lib/libmath.a

all: $(TARGET)

$(TARGET): $(SRCS)
	ar rcs $@ $(SRCS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	if len(cfg.Constants) != 1 {
		t.Fatalf("expected 1 constant, got: %v", cfg.Constants)
	}
	dep := cfg.Dependencies[cfg.Constants[0]]
	if dep.BuildType != "static" {
		t.Errorf("expected build.type 'static', got: %s", dep.BuildType)
	}
}

func TestConverter_SharedLibrary(t *testing.T) {
	input := `
CC = gcc
CFLAGS = -fPIC -Wall
SRCS = src/plugin.c src/hook.c
TARGET = lib/libplugin.so

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) -shared $(CFLAGS) -o $@ $(SRCS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	dep := cfg.Dependencies[cfg.Constants[0]]
	if dep.BuildType != "shared" {
		t.Errorf("expected build.type 'shared', got: %s", dep.BuildType)
	}
}

func TestConverter_ObjectDependency(t *testing.T) {
	input := `
CC = gcc
CFLAGS = -Wall -g
SRCS = src/app.c src/helper.c
OBJS = $(SRCS:.c=.o)
TARGET = bin/app_with_objs

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	dep := cfg.Dependencies[cfg.Constants[0]]
	if !dep.ObjectDpdcy {
		t.Errorf("expected object.dpdcy = true, got false")
	}
}

func TestConverter_FlagsAndIncludes(t *testing.T) {
	input := `
CC = gcc
CFLAGS = -Wall -Wextra -DDEBUG -Iinclude -I/usr/include/json-c
LIBS = -lm -lpthread -ljson-c
SRCS = src/main.c
TARGET = bin/service_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) $(SRCS) -o $@ $(LIBS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	c := cfg.Constants[0]
	setup := cfg.Setups[c]
	dep := cfg.Dependencies[c]

	if setup.Flags != "-Wall -Wextra -DDEBUG" {
		t.Errorf("expected clean flags without -I, got: %s", setup.Flags)
	}
	if len(dep.Includes) != 2 || dep.Includes[0] != "include" || dep.Includes[1] != "/usr/include/json-c" {
		t.Errorf("expected extracted includes, got: %v", dep.Includes)
	}
	if dep.Libs != "-lm -lpthread -ljson-c" {
		t.Errorf("expected libs '-lm -lpthread -ljson-c', got: %s", dep.Libs)
	}
}

func TestConverter_IdentifierCollisionDeduplication(t *testing.T) {
	input := `
CC = gcc
SRCS_BIN = src/bin_test.c
SRCS_PKG = src/pkg_test.c

all: bin/test pkg/test

bin/test: $(SRCS_BIN)
	$(CC) -o $@ $(SRCS_BIN)

pkg/test: $(SRCS_PKG)
	$(CC) -o $@ $(SRCS_PKG)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	if len(cfg.Constants) != 2 {
		t.Fatalf("expected 2 constants, got: %v", cfg.Constants)
	}
	if cfg.Constants[0] == cfg.Constants[1] {
		t.Errorf("expected distinct constants, got duplicates: %v", cfg.Constants)
	}
}

func TestConverter_CustomScripts(t *testing.T) {
	input := `
.PHONY: all clean run test install lint

CC = gcc
SRCS = src/app.c
TARGET = bin/app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) -o $@ $(SRCS)

run:
	./bin/app --port 8080

test:
	./bin/test_runner --verbose

install:
	mkdir -p /usr/local/bin
	cp bin/app /usr/local/bin/

lint:
	cppcheck --enable=all src/

clean:
	rm -f $(TARGET)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed: %v", err)
	}

	if cfg.Scripts["run"] != "./bin/app --port 8080" {
		t.Errorf("unexpected run script: %s", cfg.Scripts["run"])
	}
	if cfg.Scripts["test"] != "./bin/test_runner --verbose" {
		t.Errorf("unexpected test script: %s", cfg.Scripts["test"])
	}
	if cfg.Scripts["install"] != "mkdir -p /usr/local/bin && cp bin/app /usr/local/bin/" {
		t.Errorf("unexpected install script: %s", cfg.Scripts["install"])
	}
	if cfg.Scripts["lint"] != "cppcheck --enable=all src/" {
		t.Errorf("unexpected lint script: %s", cfg.Scripts["lint"])
	}
	// 'clean' and 'all' must be discarded
	if _, exists := cfg.Scripts["all"]; exists {
		t.Errorf("'all' should not be in scripts")
	}
	if _, exists := cfg.Scripts["clean"]; exists {
		t.Errorf("'clean' should not be in scripts")
	}
}

func TestConverter_GracefulFallbackUnsupportedDirectives(t *testing.T) {
	input := `
export GLOBAL_VAR = 123
unexport LOCAL_VAR
vpath %.c src
override EXTRA_FLAGS = -DEXTRA

ifeq ($(DEBUG), 1)
CFLAGS = -g -O0
else
CFLAGS = -O2
endif

include optional.mk
-include deps.mk

CC = gcc
SRCS = src/main.c
TARGET = bin/graceful_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) -o $@ $(SRCS)
`
	ast, err := converter.ParseMakefile(input)
	if err != nil {
		t.Fatalf("ParseMakefile failed: %v", err)
	}

	cfg, err := converter.ConvertAST(ast)
	if err != nil {
		t.Fatalf("ConvertAST failed on graceful input: %v", err)
	}

	if len(cfg.Constants) != 1 || cfg.Constants[0] != "graceful_app" {
		t.Errorf("expected 1 constant 'graceful_app', got: %v", cfg.Constants)
	}
}

func TestConverter_EmptyAndNilAST(t *testing.T) {
	cfg, err := converter.ConvertAST(nil)
	if err != nil {
		t.Fatalf("ConvertAST(nil) should not error: %v", err)
	}
	if len(cfg.Constants) != 0 || len(cfg.Scripts) != 0 {
		t.Errorf("expected empty config for nil AST")
	}
}
