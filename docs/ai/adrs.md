# docs/ai/adrs.md
# Architectural Decision Records (ADRs)

## ADR 1: Read-Only Host-Root Chroot (`--chroot /`)

**Context:**
When executing compiled binary code (like C) or running interpreters (like Python) under `nsjail`, the application requires access to system libraries (e.g. `libc.so`, `libpython3.11.so.1.0`) and runtime binaries. If we isolate the sandbox using a clean chroot folder containing only the user program, execution fails immediately because the dynamic linker cannot find its dependencies.

**Options considered:**
1. **Custom Minimal Chroot Directory:** Dynamically detect all shared library dependencies using `ldd` and copy them into the sandbox workspace for every execution.
2. **Host-Root Chroot with Read-Only Binds (`--chroot /`):** Use the host filesystem's `/` as the chroot path in read-only mode, and bind paths like `/lib`, `/usr`, and `/bin` read-only.

**Decision:**
Use the host root `/` as a read-only chroot combined with read-only binds, while mounting a separate writable `tmpfs` at `/tmp` for temporary workspaces.

**Rationale:**
Copying dependencies dynamically is slow, fragile, and prone to breaking when language runtimes update. Using `--chroot /` read-only keeps shared library lookups 100% correct, supports compilation pipelines out of the box, and ensures files on the host are protected since the sandbox has zero write access to anything except `/tmp`.

---

## ADR 2: Map-Based Registry Pattern via YAML

**Context:**
We need to support multiple languages with unique compilation/run arguments and limits. Hardcoding language rules within Go structs or handlers makes extending the platform difficult. Adding new runtimes should be possible without editing Go code.

**Options considered:**
1. **Hardcoded Switch/Case:** Map language behavior using Go code logic in the handlers.
2. **YAML-Driven Generic Registry:** Store all metadata, arguments, commands, and allow-lists inside a structured YAML file, loading it into a map-based lookup at server startup.

**Decision:**
Load and validate languages from `configs/languages/languages.yaml` into a `map[string]Language` registry at startup.

**Rationale:**
This separates platform logic from runtime configuration, complying with the "plug-and-play" architectural constraint. We can support additional runtimes in Stage 2 simply by editing the YAML file.

---

## ADR 3: Atomic Temporary Directories (`os.MkdirTemp`)

**Context:**
The executor writes user code to files on disk, compiles them, feeds inputs, and compares outputs. If multiple requests execute concurrently using static directory paths, they will overwrite each other's source files, causing collisions and data leaks.

**Options considered:**
1. **UID-based directories:** Generate folders named after the run ID or numeric request ID.
2. **Atomic Temp Directories (`os.MkdirTemp`):** Use the operating system's temporary folder API to create randomly-named directories atomically.

**Decision:**
Create a unique directory using `os.MkdirTemp` for every run, and use Go's `defer` to clean up the directory when the function finishes.

**Rationale:**
`os.MkdirTemp` guarantees that directory names do not collide, providing atomic isolation on the disk level. Utilizing `defer os.RemoveAll` ensures cleanup is executed on all execution paths, preventing stale files from polluting disk space.
