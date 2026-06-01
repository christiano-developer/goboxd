# Language Registry System (Stage 2)

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
  - id: java                              # Unique identifier (used in POST /run)
    name: Java                            # Human-readable name
    source_filename_strategy: from_request # Resolves source filename from client request
    artifact_filename_strategy: from_request # Resolves artifact filename from client request
    build:                                # Compilation configuration
      cmd: /usr/bin/javac                 # Compiler binary path
      args:                               # Arguments to compiler (with placeholders)
        - "-J-XX:+UseSerialGC"
        - "-J-XX:TieredStopAtLevel=1"
        - "-J-XX:CompressedClassSpaceSize=32m"
        - "-J-Xms128m"
        - "-J-Xmx256m"
        - "{{source}}"
      limits:                             # Compilation resource limits
        wall_time_s: 15
        memory_kb: 1048576
        max_processes: 100
    run:                                  # Execution configuration
      cmd: /usr/bin/java                  # Runner binary path
      args:                               # Arguments to runner
        - "-XX:+UseSerialGC"
        - "-XX:TieredStopAtLevel=1"
        - "-XX:CompressedClassSpaceSize=32m"
        - "-Xms64m"
        - "-Xmx128m"
        - "{{artifact}}"
      limits:                             # Running execution limits
        wall_time_s: 10
        memory_kb: 1048576
        max_processes: 100
```

### Placeholder Substitution Rules:
The executor maps arguments dynamically by searching for the following tokens inside `args` or execution `cmd` blocks:
* `{{source}}`: Expands to the absolute path of the written source code file (e.g. `/tmp/goboxd-123/solution.c`).
* `{{artifact}}`: Resolves relative to the working directory (`Main` or `solution`) for argument commands (e.g. `java Main`), but expands to the absolute path for execution targets (e.g. `/tmp/goboxd-123/solution`).
* `{{flags}}`: Expands to the slice of compiler flags provided by the client request (e.g., `["-O3", "-std=c++20"]`).

---

## 3. Supported Languages List

### 1. Python 3 (`py3`)
* **Type:** Interpreted
* **Engine:** `/usr/bin/python3`
* **Default Limits:** Wall time 9s · Memory 100 MiB · Processes 100 max

### 2. C (`c`)
* **Type:** Compiled
* **Compiler:** `/usr/bin/gcc`
* **Runner Target:** `./{{artifact}}`
* **Flag Allow-list:** `["-O0", "-O1", "-O2", "-Wall", "-Wextra", "-std=c11", "-std=c99"]`
* **Default Limits:** Compile: 15s/1GB · Run: 10s/125MB

### 3. C++ (`cpp`)
* **Type:** Compiled
* **Compiler:** `/usr/bin/g++`
* **Runner Target:** `./{{artifact}}`
* **Flag Allow-list:** `["-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=c++17", "-std=c++20"]`
* **Default Limits:** Compile: 15s/1GB · Run: 10s/125MB

### 4. Java (`java`)
* **Type:** Compiled
* **Compiler:** `/usr/bin/javac` (with Serial GC and JIT constraints)
* **Runner Target:** `/usr/bin/java` (with Serial GC and JIT constraints)
* **Dynamic Strategies:** Evaluates classname matching by reading `source_filename_strategy` and `artifact_filename_strategy` from request values.
* **Default Limits:** Compile: 15s/1GB · Run: 10s/1GB

### 5. Bash (`bash`)
* **Type:** Interpreted (Script)
* **Engine:** `/bin/bash`
* **Default Limits:** Run: 5s/50MB · Processes 32 max

### 6. JavaScript (`js`)
* **Type:** Interpreted (V8 Engine)
* **Engine:** `/usr/bin/node` (with `--max-old-space-size=128` heap constraints)
* **Default Limits:** Run: 9s/1GB · Processes 64 max

### 7. Verilog (`verilog`)
* **Type:** Compiled
* **Compiler:** `/usr/bin/iverilog` (compiles to `.vvp` simulator artifact)
* **Runner Target:** `/usr/bin/vvp`
* **Default Limits:** Compile: 10s/256MB · Run: 5s/125MB
