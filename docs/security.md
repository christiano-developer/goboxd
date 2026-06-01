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
* *Note:* To orchestrate jails inside a container, Docker runs with `privileged: true` or custom cap grants (`CAP_SYS_ADMIN`), making Layer 2 (nsjail) critical for workload isolation.

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
* **User Namespace (`CLONE_NEWUSER`)**: Maps the internal sandbox user (acting as `root` inside the jail) to a non-privileged UID on the host system, neutralizing privilege escalation vectors.

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

### Output Truncation
To prevent user code from flooding stdout/stderr and consuming infinite heap space, the executor reads streams via an `io.LimitReader` capped at `64 KiB`. Any output exceeding this cap is discarded, and a truncation marker is appended.
