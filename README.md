# goboxd

goboxd (Go Sandbox Daemon) is a Go HTTP service that compiles and executes untrusted code inside an isolated `nsjail` sandbox and returns per-test results.

---

## HTTP Framework Choice

We use the Go standard library's `net/http` (with the enhanced `ServeMux` introduced in Go 1.22) for all endpoint routing. This design avoids introducing third-party web framework dependencies and guarantees excellent performance with zero external overhead.

---

## Features

*   **Nested Sandboxing:** Dual-layer containment using Docker and `nsjail` (utilizing user/UTS/PID/IPC namespaces, tmpfs mounts, process caps, and disabled networking).
*   **Plug-and-Play Languages:** Declarative runtime toolchain specs loaded dynamically via YAML, featuring custom smoke check configuration commands.
*   **SJF Concurrency Queue:** Min-heap request scheduling sorted by execution cost, integrated with starvation prevention (wait-time aging) and graceful server shutdown.
*   **Load-Adaptive Resource Clamping:** Monitors request rate over a sliding window, dynamically scaling down CPU and memory limits to protect the host under heavy load.
*   **Process-Unique UID Isolation:** Maps concurrent execution tasks to process-unique unprivileged UIDs, preventing sibling process and workspace directory collisions.
*   **Structured telemetry:** Exposes real-time queue states, active jobs, disk capacity, and registry metadata under `/info`.

---

## Documentation

Extended documentation is located in the `docs/` directory:

*   **[API Reference](docs/api.md)** — Payload details for `/run`, `/healthz`, `/readyz`, and `/info`.
*   **[System Architecture](docs/architecture.md)** — Concurrency scheduling, priority queueing, and lifecycle flows.
*   **[Language Registry](docs/languages.md)** — Configuration schema for registering compilers and runtimes.
*   **[Security Model](docs/security.md)** — Explaining namespace isolation, UID mapping, and resource limits.

---

## Configuration

The service can be configured using the following environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `CONCURRENCY_LIMIT` | Maximum concurrent sandboxes executing at once. | `runtime.NumCPU()` |
| `MAX_QUEUE_SIZE` | Maximum pending requests allowed in the priority queue. | `500` |
| `LANGUAGE_CONFIG` | Path to the registered languages YAML file. | `configs/languages/languages.yaml` |

---

## Getting Started

### Prerequisites

*   Docker (with compose v2)

### Booting the Server

1. **Build the container image:**
   ```bash
   make build
   ```

2. **Run the HTTP service (listens on port 8080):**
   ```bash
   make run
   ```

---

## Command Reference

Every common operation has a dedicated Makefile target:

*   `make build` — Builds the goboxd runtime image.
*   `make run` — Spins up the goboxd daemon on port 8080.
*   `make test` — Runs unit and configuration validation tests.
*   `make integration` — Runs end-to-end sandbox execution tests inside Docker.
*   `make load` — Launches a local performance load test against a running server.
*   `make lint` — Runs static analysis and code checks (`golangci-lint`).
