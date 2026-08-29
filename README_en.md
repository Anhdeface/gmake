# Gomake

**Version: 0.1.1 (Beta)**

[Vietnamese](README.md) | English

Gomake is a transpiler written in Go that parses simplified configuration files (`.gomake`) and generates standard GNU Makefiles.

## Software Architecture

The project is structured into three main modules:
- **Parser (`internal/parser`)**: Reads `.gomake` files line by line, excludes inline comments starting with `//`, and extracts keys and values from the `[config.setup]` and `[config.dependency]` blocks. The parsing execution halts upon encountering the `./gomake` EOF marker.
- **Generator (`internal/generator`)**: Consumes the parsed data structure and outputs a formatted Makefile. The generator handles directory modifications (e.g., prefixing `includes` with `-I`) and sets up object file linking rules via the `$(OBJS)` variable.
- **CLI Router (`main.go`)**: Manages command-line arguments, orchestrates concurrent file processing using `sync.WaitGroup`, and handles the template generation routine.

## Installation

Execute the following command to build the binary from source:

```sh
go build -o gomake main.go
```

## Usage

### 1. Configuration Template Generation

This command outputs a `build.gomake` file containing empty parameters and technical comments:

```sh
./gomake genconfig
```

### 2. Single File Transpilation

Compiles a specific `.gomake` file. The output will be written to a new file appended with the `.makefile` extension (e.g., `filename.gomake.makefile`):

```sh
./gomake <filename.gomake>
```

### 3. Concurrent Batch Processing

Triggers Goroutines to locate and transpile all files with the `.gomake` extension in the current working directory concurrently:

```sh
./gomake all
```

## Configuration Specification

The format architecture utilizes logical blocks delimited by square brackets. The parser strictly preserves parameter integrity (e.g., compiler flags are exact strings without implicit additions).

- `[config.setup]`: Stores compiler designation, compilation flags, and the project name.
- `[config.dependency]`: Stores target output definitions, source paths (supporting wildcard globbing), include directories, and object linking states (`object.dpdcy`).
- `//`: Denotes a comment string.
- `./gomake`: Denotes the configuration EOF marker.

Example `.gomake` file format:

```
[config.setup]
compiler = gcc
flags = -Wall -Wextra -O2
name = my_app
[end]
[config.dependency]
target = bin/program
sources = src/main.c src/driver.c
includes = include/*
object.dpdcy = yes
[end]
./gomake
```
