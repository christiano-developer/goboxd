# docs/ai/prompts.md
# AI Prompts Log (Stage 1)

## 2026-05-31 · Stage 1 Build sequence & Workspace Setup

**Prompt:**
Where do I start and what do I need to complete?

**Response summary:**
Identified empty folders marked with `.gitkeep` files and defined an 11-step build order to safely bring up Stage 1: `main.go` entrypoint → models → languages config loader → YAML language definitions → executor → handler → Dockerfile updates → unit tests → documentation → linting → PR opening.

**What we used / didn't use:**
Used the proposed 11-step sequence as our checklist to maintain a steady, testable flow of commits.

---

## 2026-05-31 · Go project packages and directory structure

**Prompt:**
How should I efficiently structure files and folders?

**Response summary:**
Confirmed that standard Go layouts (`cmd/` for entrypoints, `internal/` for private application modules, `configs/` for language configuration) were ideal, keeping package names aligned with folder names.

**What we used / didn't use:**
Adopted the clean structure layout, keeping packages decoupled (e.g. `validate`, `executor`, `languages`).

---

## 2026-05-31 · API spec alignment with full competition brief

**Prompt:**
Pasted the full competition brief. Do we need external libraries/packages, and are there any differences between this brief and the initial plans?

**Response summary:**
Stated that no third-party libraries were required beyond `gopkg.in/yaml.v3` for configuration. Flagged key specification updates from the brief: field names (`source` instead of `code`), new `/readyz` and `/info` endpoints, required security mitigations, and performance tests. Updated the build checklist to 16 steps.

**What we used / didn't use:**
Used standard library packages only, renamed fields to match the final spec, and expanded our task checklist.

---

## 2026-05-31 · Stage timeline vs. specs resolution

**Prompt:**
Confused about what to build when — stage spec vs. stage timeline?

**Response summary:**
Clarified that Stage 1 corresponds to the June 1 EOD deadline and requires a basic working prototype with C/Python runtimes. Suggested deferring `/readyz`, `/info`, concurrency pools, and load testing to Stage 2/3. Trimmed the build sequence to 13 steps.

**What we used / didn't use:**
Deferred endpoints and limits, focusing strictly on core POST `/run` support for Python and C.

---

## 2026-05-31 · cmd/goboxd/main.go HTTP server and health check

**Prompt:**
Start Step 1: cmd/goboxd/main.go.

**Response summary:**
Provided Go code to start a basic HTTP server on port `:8080` returning "ok" on `GET /healthz`. Suggested using `http.NewServeMux` with Go 1.22 routing pattern and `log/slog`.

**What we used / didn't use:**
Implemented the HTTP server and routing. We initially returned a plain text string "ok", but corrected it in a later step to return JSON `{"status":"ok"}`.

---

## 2026-05-31 · Designing model structs, language registry, and YAML loader

**Prompt:**
Guide through model structs and language registry.

**Response summary:**
Proposed structures for `RunRequest` and `RunResponse` JSON schemas, along with the `Language` and `PhaseConfig` YAML mapping structs. Provided a map-based parser for `languages.yaml`.

**What we used / didn't use:**
Used the exact JSON and YAML struct definitions. Mounted configuration lookup as a map of `Language` objects keyed by identifier.

---

## 2026-05-31 · Executor pipeline & validation mechanisms

**Prompt:**
Continue with executor and handler.

**Response summary:**
Proposed an execution architecture: creating unique work folders, executing compilation and run stages sequentially, reading standard output streams safely, and validating requests (filenames, code size, test suite bounds).

**What we used / didn't use:**
Used the suggested multi-stage execution pipeline and implemented validation rules to block path traversal.

---

## 2026-05-31 · Wiring POST /run handler into Go HTTP Server

**Prompt:**
Wire executor into the HTTP handler.

**Response summary:**
Provided `internal/handler/handler.go` implementation to parse requests, enforce standard constraints (size bounds, limits), execute the run module, and handle exceptions.

**What we used / didn't use:**
Wired the handler into `main.go`. Disallowed requests missing the `language` field or containing invalid filename schemas.

---

## 2026-05-31 · Dockerfile runtime dependency adjustment (Python)

**Prompt:**
Dockerfile currently missing python3 and gcc in runtime stage.

**Response summary:**
Suggested adding `python3` and `gcc` packages to the `apt-get install` block in stage 3, and copy configuration files to `/configs` while setting the `LANGUAGE_CONFIG` environment variable.

**What we used / didn't use:**
Modified the Dockerfile runtime layer and verified that the container successfully loaded `languages.yaml` upon launch.

---

## 2026-05-31 · Unit testing strategy (validate, registry, capped writer)

**Prompt:**
Write unit tests.

**Response summary:**
Supplied boilerplate for `validate_test.go` (asserting filename boundary checks and size constraints) and `registry_test.go` (verifying YAML loading behavior).

**What we used / didn't use:**
Used the unit test structures to cover core packages, running them successfully with `make test`.

---

## 2026-05-31 · Debugging nsjail dynamic linker library loading errors

**Prompt:**
POST /run failing with "error while loading shared libraries: libpython3.11.so.1.0".

**Response summary:**
Diagnosed that nsjail's chroot is isolating the process from the host's `/lib` and `/usr` libraries. Suggested mounting the root directory read-only (`--chroot /`) and binding libraries (`--ro-bind /lib`, `--ro-bind /usr`).

**What we used / didn't use:**
Used the `--chroot /` read-only mount approach. Modified the sandboxed flags to bind system runtime directories directly.

---

## 2026-05-31 · C Compilation Error (missing stdio.h and libc6-dev)

**Prompt:**
C run returning "fatal error: stdio.h: No such file or directory".

**Response summary:**
Identified that `gcc` on Debian does not include base C headers. Recommended adding `libc6-dev` to the runtime container installation packages.

**What we used / didn't use:**
Installed `libc6-dev` in the runtime stage of the Dockerfile. Rebuilt the image, successfully compiling and executing C programs.

---

## 2026-05-31 · Stage 1 Completion check & documentation prep

**Prompt:**
We've basically completed Stage 1 requirements.

**Response summary:**
Summarized deliverables achieved (health check JSON, Python/C runs, tests passing, linting clean). Outlined the next phase documentation requirements (`README.md`, `api.md`, `architecture.md`, `languages.md`, `security.md`).

**What we used / didn't use:**
Accepted the summary and prepared to start writing the comprehensive documentation files.

---

## 2026-06-01 · Stage 3 Bounded Concurrency, Dynamic Scheduling, and Load-Adaptive Limits

**Prompt:**
Work on the concurrency and sustained load part. Discuss Docker container resource allocations, enforce minimum memory limits to prevent boot failures for Java and JavaScript, implement priority queueing (Shortest Job First with Starvation Aging), overload queue size limit at 500, dynamic Retry-After header calculations, and graceful shutdown.

**Response summary:**
Provided architecture and design details:
- Bounded Concurrency: Channel-based semaphore to limit active jobs to 15, and Priority Queue (using `container/heap`) to hold up to 500 waiting requests.
- SJF + Aging: Expected wall time cost score with wait-time aging subtractions to prevent starvation.
- Under/Over-Allocation Clamping: Java/JS runs clamped to minimum 1 GB, other overrides clamped to safe maximums. Clamping modifications returned via a `warnings` field.
- Load-Adaptive Capping: Sliding window tracker dynamically lowers maximum resource overrides when request rate exceeds 5 req/sec.
- Graceful Shutdown: Captured SIGINT/SIGTERM to trigger `http.Server.Shutdown`.
Implemented the changes across `pool.go`, `validate.go`, `handler.go`, `info.go`, and `main.go`.

**What we used / didn't use:**
Implemented all proposed changes. Verified with new test cases covering rate tracking, priority sorting, aging, and load-shedding.
