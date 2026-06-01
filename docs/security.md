# Security Architecture & Sandbox Boundaries (Stage 2)

This document describes the security model, isolation mechanisms, and defense-in-depth layout of the **goboxd** sandbox service.

---

## 1. Dual-Layer Security Model

The service implements a defense-in-depth model, wrapping executing user code inside two distinct sandbox layers:

```
+--------------------------------------------------------+
| Host Machine                                           |
|  +--------------------------------------------------+  |
|  | Layer 1: Docker Container (Service Runtime)      |  |
|  |   +--------------------------------------------+ |  |
|  |   | Layer 2: nsjail Sandbox (Execution Jail)   | |  |
|  |   |   +------------------------------------+   | |  |
|  |   |   | Untrusted User Code                |   | |  |
|  |   |   +------------------------------------+   | |  |
|  |   +--------------------------------------------+ |  |
|  +--------------------------------------------------+  |
+--------------------------------------------------------+
```

### Layer 1: Docker Container Boundaries
* The `goboxd` Go server and the compiler/interpreter toolchains execute inside a Debian container.
* The container isolates the compilation tools and dependencies from the host machine.
* **Minimal Privileges (Host Security Hardening)**: The container is configured with the specific `SYS_ADMIN` capability (`cap_add: [SYS_ADMIN]`) along with unconfined Seccomp/AppArmor security options, instead of the broad and insecure `privileged: true`.
* **Procfs Overmount Bypass (`--disable_proc`)**: Inside non-privileged containers, Docker mounts "masked" paths over `/proc` (e.g., `/proc/kcore` mapped to `/dev/null`) to block host kernel leaks. Under Linux namespace rules, creating a nested namespace and mounting a new `procfs` over a masked procfs is blocked with `Operation not permitted`. We bypassed this restriction cleanly by passing the `--disable_proc` flag to `nsjail`, which completely disables procfs mounting inside the jail. Compilers and interpreters execute successfully without `/proc`.

### Layer 2: nsjail Sandboxing
`nsjail` is a lightweight, secure sandboxing tool utilizing Linux kernel features (namespaces, cgroups, seccomp filters) to run processes under strict resource constraints.

---

## 2. nsjail Isolation Mechanisms

### Namespace Isolation
Every code execution runs in its own clean set of Linux namespaces:
* **PID Namespace (`CLONE_NEWPID`)**: The sandboxed program cannot see any other processes running on the system or container. It runs as PID 1 inside its namespace.
* **Mount Namespace (`CLONE_NEWNS`)**: The jail operates on an independent file system mount hierarchy.
* **Network Namespace (`CLONE_NEWNET`)**: Disables network access inside the sandbox, preventing user code from making outbound connections or running reverse shells.
* **IPC Namespace (`CLONE_NEWIPC`)**: Prevents communication via system-level IPC pipelines (shared memory, message queues).
* **UTS Namespace (`CLONE_NEWUTS`)**: Isolates hostnames.
* **User Namespace (`CLONE_NEWUSER`) & UID Isolation**: Maps the internal sandbox user (acting as `root` inside the jail) to a process-unique, unprivileged UID on the host system, neutralizing privilege escalation vectors. 
  To prevent UID collisions under concurrent load, `goboxd` allocates a unique UID for each request using a thread-safe atomic counter and process ID:
  $$\text{UID} = 100000 + (\text{PID} \bmod 100000) \times 10000 + (\text{counter} \bmod 10000)$$
  This guarantees that concurrent requests run under strictly distinct UIDs, preventing them from interacting with each other's processes or accessing sibling temp directories (which are set to `0777` permissions to allow access to the unprivileged UID).

### Filesystem Isolation
* **Read-Only Root (`--chroot /`)**: The host root filesystem is mounted as read-only. User code cannot modify compiler libraries, interpreters, or configuration files.
* **Writable Workspaces (`--tmpfsmount /tmp`)**: A virtual `tmpfs` is mounted at `/tmp`. Code writing, compilation, and execution occur strictly inside memory-backed, ephemeral space that is discarded instantly when the sandbox exits.

### Resource Limits (Cgroups & Rlimits)
`nsjail` sets strict boundaries on CPU, memory, and process creation to prevent Denial of Service (DoS) attacks:
* **Time limits (`--time_limit`)**: Enforces execution timeouts (wall-time). If execution hangs or runs into infinite loops, `nsjail` kills the process.
* **Process limits (`--max_pids` / `--rl_nproc`)**: Limits the number of concurrent processes or threads the sandbox can spawn, preventing fork bomb attacks.
* **Memory Limits (`--rlimit_as` / `--max_memory_m`)**: Limits address space allocation. Out-of-memory executions are instantly terminated.

---

## 3. Virtual Machine Memory Defense

Modern execution runtimes (like the Java JVM and Node.js V8 engine) attempt to reserve large virtual address spaces (often 1 GB or more) during startup for heap, GC card tables, and compressed class spaces. Under strict sandbox address limits, this results in immediate startup crashes (`OutOfMemoryError` or VM allocation failures).

`goboxd` implements a **dual defense** for VM runtimes:
1. **Virtual Space Overhead Allowances:** Raised memory allocation caps for JVM and Node.js sandboxes to 1 GB.
2. **Runtime Memory Optimization Flags:** 
   * **Java compiler/runner:** Configured to use the Serial Garbage Collector (`-XX:+UseSerialGC`) and level 1 tiered JIT compilation (`-XX:TieredStopAtLevel=1`), which minimizes thread stack and compilation table metadata overhead. We also restrict the compressed class space size (`-XX:CompressedClassSpaceSize=32m`) to reduce the default 1 GB address space reservation to 32 MB.
   * **Node.js runner:** Configured to limit the V8 old space heap memory allocation (`--max-old-space-size=128`), keeping V8 from over-reserving memory blocks.

This ensures that the runtimes boot successfully and execute with minimal physical memory (RSS) footprints, protecting the server against host memory exhaustion.

---

## 4. Host-Level Defenses

### Path Traversal Mitigation
The HTTP validator package (`internal/validate`) blocks request filenames containing path separators (`/`, `\`) or dot prefixes (`.`, `..`). This ensures user source files are written strictly inside the unique workspace directory created by `os.MkdirTemp`, making directory traversal out of the workspace boundary impossible.

### Strict Request Size Limits
To prevent denial of service (DoS) and memory exhaustion attacks at the HTTP parsing layer, we enforce strict boundaries:
* **HTTP Body Capping**: The total HTTP request body read is capped at **4 MiB** (`MaxRequestBodyBytes`) in the handler.
* **Component-Level Limits**:
  * **Source Code size**: Capped at **256 KiB** (`MaxSourceBytes`).
  * **Test Count**: Capped at **50** max cases (`MaxTests`).
  * **Per-Test Inputs**: Each test case's `Stdin` and `ExpectedStdout` are validated to not exceed **64 KiB** (`MaxStdinBytes`).

### Output Truncation
To prevent runaway programs inside the sandbox from flooding stdout/stderr and OOM-ing the Go host process during response capture, the executor limits the read buffer size. Streams are read via an `io.LimitReader` capped at **64 KiB** (`maxOutputBytes`). Any output exceeding this cap is discarded, and a truncation marker is appended.

### Stale Jail Directory Cleanup
To prevent disk exhaustion from orphaned sandbox directories:
* **Exit-Path Cleanup**: Every exit path in execution runs within a `defer os.RemoveAll(workDir)` scope to clean up workspaces immediately.
* **Startup Orphan Sweep**: Upon server boot, a background garbage collection sweep (`executor.SweepOrphans`) scans `/tmp` and removes any stale `goboxd-*` directories older than **5 minutes**.
