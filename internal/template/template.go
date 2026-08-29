package template

const ConfigTemplate = `// ==========================================
// Gomake Configuration Template
// Use '//' for comments
// ==========================================

[config.setup]
// compiler: The compiler you want to use (e.g., gcc, clang, g++)
compiler = 

// flags: Compilation flags (e.g., -Wall -Wextra -O2, -g)
flags = 

// name: The name of your project
name = 
[end]

[config.dependency]
// target: The path or name of the output executable (e.g., bin/my_program)
target = 

// sources: Source code files (e.g., src/*.c, src/main.c src/utils.c)
sources = 

// includes: Directories containing header files (e.g., include/*)
includes = 

// object.dpdcy: Automatically manage and link object (.o) files. Set to 'yes' to enable (disabled by default).
object.dpdcy = 

// build.type: The type of output to build. Expected 'executable', 'static' (e.g., .a), or 'shared' (e.g., .so). Defaults to 'executable'.
build.type = 

// libs: Linker flags and libraries (e.g., -lm -lpthread -L/usr/lib -lfoo)
libs = 
[end]

[config.scripts]
// Define custom scripts here (e.g., run, test, install)
// Example: run = ./bin/my_program --debug
[end]

// The line below signals the end of the configuration file and starts the build process
./gomake
`
