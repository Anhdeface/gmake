# Gomake

**Version: 0.2.1 (Beta)**

[Vietnamese](README.md) | [English](README_en.md)

**Gomake** is a lightweight, zero-dependency bi-directional transpiler and converter written in Go. It enables you to declare C/C++ project build structures using intuitive, INI-style configuration files (`.gomake`) and produces clean, robust, multi-target **GNU Makefiles** with out-of-the-box automatic header dependency tracking.

---

## Why Gomake?

Writing and maintaining raw Makefiles is notoriously error-prone:
* Subtle syntax bugs caused by confusing Tab and Space characters.
* Missing automatic header dependency tracking (`-MMD -MP`), forcing developers to constantly clean and recompile whenever `.h` files change.
* Object file (`.o`) collisions across multiple targets sharing similar filenames (such as `main.c` or `utils.c`).
* Complex build systems like CMake or Meson introduce significant overhead for small-to-medium C/C++ projects, embedded firmware, or command-line utilities.

Gomake resolves these issues through a straightforward configuration format that reliably compiles into clean GNU Makefiles.

---

## Features

* **Bi-directional Engine**:
  * Transpile `.gomake` configurations into standard GNU Makefiles.
  * Reverse-convert existing `Makefile`s into `.gomake` via a standalone static analysis parser.
* **Automatic Header Tracking**: Automatically injects `-MMD -MP` and `-include *.d` directives to track every header change (`.h`/`.hpp`) for dependable incremental builds.
* **Zero Object Collisions**: Employs target-namespaced object files (`.target.o`) so multiple targets never overwrite each other's compiled objects.
* **Multi-Artifact Support**: Native generation for Executables, Static Libraries (`.a`), and Shared Libraries (`.so`).
* **Concurrent Processing**: Batch-transpiles all `.gomake` files in parallel using Go Goroutines (`gomake all`).
* **Zero Dependencies**: Written purely with the Go Standard Library. Produces a single self-contained binary with no external runtime requirements.

---

## Architecture

The codebase is organized into modular components:
* `internal/parser`: Line-by-line `.gomake` parser. Manages multi-target `[const]` lists, preserves script commands, and stops at the `./gomake` EOF marker.
* `internal/generator`: Consumes the parsed configuration and emits idiomatic GNU Makefiles with compiler flags, include directories, library linking, and cleanup rules.
* `internal/converter`: A standalone parser and AST engine featuring a Lexer, AST Builder, and Variable Expander supporting 25+ GNU Make functions (including `wildcard`, `patsubst`, conditional directives `ifeq`/`ifdef`, and automatic variables). Responsible for reverse-converting Makefiles into `.gomake`.
* `main.go`: CLI command dispatcher with concurrent execution managed via `sync.WaitGroup`.

---

## Installation

### Method 1: Build from Source
Requires Go installed on your system:
```sh
git clone https://github.com/Anhdeface/gmake.git
cd gmake
./build.sh
```

### Method 2: Install via Go CLI
```sh
go install github.com/Anhdeface/gmake@latest
```

---

## Usage Guide

### 1. Generate a Starter Template
Generate a sample `build.gomake` file with two predefined targets (`app` and `test`):
```sh
./gomake genconfig
```

### 2. Transpile a Single Configuration
Compile a `.gomake` file into a Makefile (default output: `<filename>.makefile`):
```sh
./gomake build.gomake
# Outputs: build.gomake.makefile
```
Run directly with GNU Make:
```sh
make -f build.gomake.makefile
```

### 3. Batch Transpilation
Find and transpile all `*.gomake` files in the current working directory concurrently:
```sh
./gomake all
```

### 4. Reverse-Convert Existing Makefile to Gomake (convert)
Analyze an existing Makefile and convert it into a native `.gomake` configuration:
```sh
./gomake convert -i Makefile -o build.gomake -f
```

Supported flags for `convert`:
| Flag | Shorthand | Default | Description |
|---|---|---|---|
| `--input` | `-i` | `Makefile` | Path to input Makefile |
| `--output` | `-o` | `build.gomake` | Path to output `.gomake` file |
| `--force` | `-f` | `false` | Overwrite output file if it exists |
| `--stdout` | `-s` | `false` | Output converted configuration directly to stdout |
| `--verbose` | `-v` | `false` | Enable verbose diagnostic output |

*Technical Note:* The converter reliably translates standard Make patterns (compilers, flags, dependencies, includes, libraries, and recipes). For overly complex or dynamic constructs (such as multi-level dynamic `$(eval)` or custom pattern rules), the converter applies a graceful fallback strategy to keep the resulting configuration file simple, readable, and maintainable.

---

## .gomake Syntax Specification

Configurations use bracketed logical blocks `[...]` and must end with `./gomake`:

```ini
[const]
app, test

[config.setup.app]
compiler = gcc
flags = -Wall -O2
name = my_app
[end]

[config.dependency.app]
sources = src/*.c
includes = include/
object.dpdcy = yes
build.type = executable
libs = -lm -lpthread
[end]

[config.scripts]
run = ./my_app
[end]

./gomake
```

### Configuration Fields Reference:
* `[const]`: Comma-separated list of target identifiers declared on one line.
* `[config.setup.<TARGET>]`:
  * `compiler`: Toolchain compiler (e.g., `gcc`, `g++`, `clang`). Default: `gcc`.
  * `flags`: Compiler flags (e.g., `-Wall -O3 -std=c11`).
  * `name`: Output binary or library filename.
* `[config.dependency.<TARGET>]`:
  * `sources`: Source code files (supports wildcards `*`, e.g. `src/*.c`).
  * `includes`: Header directory paths (automatically formatted as `-I<dir>`).
  * `object.dpdcy`: Set to `yes` to compile individual object files with automatic header tracking (`-MMD -MP`).
  * `build.type`: Artifact type: `executable` (default), `static` (`.a` archive), or `shared` (`.so` library).
  * `libs`: Linker flags and libraries (e.g., `-lm -lpthread`).
* `[config.scripts]`: Custom commands (e.g., `run = ./my_app`), generated as `.PHONY` targets in the Makefile.
* `./gomake`: Strict end-of-file terminator (required).

---

## Best For

* Small-to-medium C/C++ applications.
* Embedded systems and microcontroller firmware using GCC toolchains.
* Command-line utilities (CLI) and system tools.
* Academic and training environments that need standard Makefiles without manually managing Make syntax.

---

## Quality Assurance & Testing

Gomake includes a test suite with over 2,800 lines of test code:
* **Unit Tests**: Granular tests for Lexer, Parser, AST, Serializer, and Converter.
* **Stress Tests**: Recursion depth handling, circular reference prevention, and token boundary cases.
* **4-Tier E2E Verification Pipeline**:
  * Tier 1: Feature coverage (Executable, Static Library, Shared Library, Header Tracking).
  * Tier 2: Boundary conditions (Mixed whitespace/tabs, trailing slashes, inline semicolons, duplicate target names).
  * Tier 3: Multi-target combinations (Mixed static, shared, and executable builds).
  * Tier 4: Real-world projects (CLI logger application, cryptography library, multi-module daemon).

Run all tests:
```sh
go test -v ./...
```

---

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.


