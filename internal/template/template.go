package template

import (
	"fmt"
	"os"
)

const ConfigTemplate = `[const]
app, test

[config.setup.app]
// compiler: The compiler to use. E.g., gcc, g++, clang, etc. Defaults to 'gcc'.
compiler = gcc

// flags: Compilation flags. E.g., -Wall, -O2, -g.
flags = 

// name: The default target executable/library name if 'target' in [config.dependency] is not set.
name = app
[end]

[config.dependency.app]
// target: The specific output file name (overrides 'name' in setup).
target = 

// sources: Source files to compile. Use space to separate multiple files. E.g., src/*.c src/*.cpp.
sources = 

// includes: Include directories. E.g., include/ libs/
includes = 

// object.dpdcy: Automatically manage and link object (.o) files. Set to 'yes' to enable (disabled by default).
object.dpdcy = 

// build.type: The type of output to build. Expected 'executable', 'static' (e.g., .a), or 'shared' (e.g., .so). Defaults to 'executable'.
build.type = 

// libs: Linker flags and libraries (e.g., -lm -lpthread -L/usr/lib -lfoo)
libs = 
[end]

[config.setup.test]
compiler = gcc
flags = -g -Wall
name = test_runner
[end]

[config.dependency.test]
sources = test/*.c
object.dpdcy = yes
build.type = executable
[end]

[config.scripts]
run = ./app
[end]

./gomake
`

func GenerateConfig(filepath string) error {
	if _, err := os.Stat(filepath); err == nil {
		return fmt.Errorf("file %s already exists", filepath)
	}

	err := os.WriteFile(filepath, []byte(ConfigTemplate), 0644)
	if err != nil {
		return fmt.Errorf("could not create template: %w", err)
	}
	return nil
}
