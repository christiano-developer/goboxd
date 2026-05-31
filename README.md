<div align="center">

# goboxd
##A SEEK hackathon at Paradox, IIT Madras · 2026
###Christiano Fernandes

**A Go HTTP service for executing untrusted code in isolated sandboxes.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)

</div>

---

## Overview

goboxd is an HTTP service written in Go that compiles and runs untrusted code inside isolated sandboxes and returns the result. Optional test cases can be supplied to assert behaviour against expected output. It is built for safe execution of code across many languages, with strict isolation, bounded concurrency, and a plug and play language registry.

## Documentation

For detailed guides, please refer to the following documents in the `docs/` folder:

* **[API Reference](docs/api.md)** — JSON payloads, endpoint routing, and validation.
* **[System Architecture](docs/architecture.md)** — Lifecycle of a request, package structure, and designs.
* **[Language Registry](docs/languages.md)** — Dynamic execution configuration and runtime YAML schema.
* **[Security & Sandboxing](docs/security.md)** — Explaining namespaces, mounts, cgroups, and system protections.

## Getting started

### Prerequisites

- Docker with Compose v2

No Go toolchain or system dependencies are required on the host. Everything runs in containers.

### Installation

```sh
git clone https://github.com/thesouldev/goboxd.git
cd goboxd
make build
```

### Usage

```sh
make run          # start the service on :8080
make test         # run unit tests
make integration  # run end to end tests
make lint         # run static analysis
```

## License

This project is distributed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for the full text.
