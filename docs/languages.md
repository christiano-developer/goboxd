# Language Registry System (Stage 1)

This document describes how **goboxd** dynamically parses, registers, and executes languages using a configuration-driven registry system.

---

## 1. Core Concepts

The system is designed around a **zero Go modification** goal for adding new languages. All compile and execution behaviors are declared in a central YAML configuration file: `configs/languages/languages.yaml`. 

At server startup, the file is read, validated, and cached in a thread-safe registry module.

---

## 2. Configuration Schema

Each language block inside the registry contains the following parameters:

```yaml
languages:
  - id: c                         # Unique identifier (used in POST /run)
    name: C                       # Human-readable name
    source_filename: solution.c   # Target filename when writing code to disk
    artifact: solution            # Target output name for compiled binaries (compiled only)
    build:                        # OPTIONAL compilation phase configuration
      cmd: /usr/bin/gcc           # Compiler path
      args:                       # Argument list (supports placeholders)
        - "{{source}}"
        - "-o"
        - "{{artifact}}"
        - "-std=c11"
      limits:                     # Compilation resource limitations
        wall_time_s: 15
        memory_kb: 1048576
        max_processes: 100
      flag_allowlist:             # Allowlisted compiler parameters (checked if client provides overrides)
        - "-O0"
        - "-O1"
        - "-O2"
        - "-Wall"
        - "-Wextra"
        - "-std=c11"
        - "-std=c99"
    run:                          # REQUIRED execution phase configuration
      cmd: "./{{artifact}}"       # Executable path (interpreted runtimes point to interpreter binary)
      limits:                     # Execution resource limitations
        wall_time_s: 10
        memory_kb: 128000
        max_processes: 32
```

### Placeholders Substitution
The executor maps arguments dynamically by searching for the following tokens inside `args` or execution `cmd` blocks:
* `{{source}}`: Expands to the absolute path of the written source code file (e.g. `/tmp/goboxd-123/solution.c`).
* `{{artifact}}`: Expands to the absolute path of the compiled binary target (e.g. `/tmp/goboxd-123/solution`).

---

## 3. Stage 1 Supported Languages

### 1. Python 3 (`py3`)
* **Type:** Interpreted
* **Engine:** `/usr/bin/python3`
* **Default Limits:**
  * Wall time: 9 seconds
  * Memory: 100 MiB (`102400 KB`)
  * Processes: 100 max

### 2. C (`c`)
* **Type:** Compiled (Compile to binary → run test suite)
* **Compiler:** `/usr/bin/gcc`
* **Default Compiler Limits:**
  * Wall time: 15 seconds
  * Memory: 1 GiB (`1048576 KB`)
  * Processes: 100 max
* **Default Runner Limits:**
  * Wall time: 10 seconds
  * Memory: ~125 MiB (`128000 KB`)
  * Processes: 32 max
