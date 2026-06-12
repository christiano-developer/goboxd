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

---

## 2026-06-12 · Plug-and-play language test: adding PHP

**Prompt:**
Test the plug-and-play registry by adding PHP, and append the load tester (`scripts/loadtest.go`) for PHP too.

**Response summary:**
Confirmed the registry is pure-YAML, so a new language needs no Go changes — only a `languages.yaml` entry plus the runtime toolchain in the Docker image. Added an interpreted `php` entry (`/usr/bin/php {{source}}`), a `php` payload to the load tester, and wired `php` into the `mixed` rotation and the usage strings. Noted that `/readyz` smoke-checks new languages automatically via its `--version` fallback (no handler change needed). PHP was already present in the runtime Dockerfile stage.

**What we used / didn't use:**
Used the YAML-only path. PHP ran first try end-to-end (`accepted`, `hello`). Confirmed the principle that interpreted languages are effectively zero-friction to add.

---

## 2026-06-12 · Adding Kotlin and Lisp — three sandbox issues, all solved without touching Go

**Prompt:**
Add two more languages, Kotlin and Lisp, and keep it plug-and-play (no edits to the core Go files).

**Response summary:**
Added `kotlin` (compiled: `kotlinc` build → `java` run) and `lisp` (SBCL, interpreted) YAML entries, installed `sbcl` and the Kotlin compiler in the runtime Docker stage, and extended the load tester. The first end-to-end run surfaced three distinct sandbox failures, each diagnosed by reproducing the exact `nsjail` invocation inside the container:

1. **Lisp — `os_alloc_gc_space ... ENOMEM`.** SBCL reserves ~1 GB of *virtual* address space at boot; nsjail's `--rlimit_as` (derived from `memory_kb`) was far below it. Fix: bound SBCL's heap with `--dynamic-space-size 256` and raise `memory_kb` so the virtual reservation fits.
2. **Kotlin build — silent failure, then `java.io.IOException: Map failed`.** The compiler JVM reserves multiple GB of virtual space. Mirrored the existing Java entry's `-J-XX` flags (`+UseSerialGC`, `CompressedClassSpaceSize`, bounded `-Xmx`) to collapse the reservation under `rlimit_as`.
3. **Kotlin build — `Caused by: java.io.IOException: File too large`.** The decisive clue. `-include-runtime` bundles `kotlin-stdlib` into a ~4.7 MB fat jar, but nsjail defaults `RLIMIT_FSIZE` to **1 MB**, so the sandbox could not write the artifact. Since the core Go (`sandbox/nsjail.go`, which would set `--rlimit_fsize`) was off-limits, we *removed* the unwanted bundled binary instead: drop `-include-runtime` to emit a ~1 KB thin jar, and load the host-installed `kotlin-stdlib.jar` (read-only, exempt from `RLIMIT_FSIZE`) on the classpath at run time via `-cp {{artifact}}:/opt/kotlinc/lib/kotlin-stdlib.jar SolutionKt`.

Iterated by `docker cp`-ing the edited `languages.yaml` into the running container and restarting (registry reloads at startup), avoiding a full rebuild per attempt.

**What we used / didn't use:**
Used the config-only approach throughout — a proposed `--rlimit_fsize` addition to `sandbox/nsjail.go` was explicitly rejected to keep the change plug-and-play, so we worked around the 1 MB cap by shrinking the artifact. All three languages ended `accepted`. Open caveat recorded: the 1 MB `RLIMIT_FSIZE` is sidestepped, not fixed — a future language whose sandbox-written artifact exceeds 1 MB will need the Go change.

---

## 2026-06-12 · Load-testing all 10 languages

**Prompt:**
Use the load test file and test.

**Response summary:**
Ran `scripts/loadtest.go` against the live server: per-language passes for `php`, `lisp`, `kotlin` (12 requests each, concurrency 4), then a `mixed` pass (60 requests, concurrency 6) exercising all ten languages plus the random under/over-allocation override injection. PHP ~18 ms p50, Lisp ~12 ms p50, Kotlin ~2.2 s p50 (JVM compile dominates). All individual runs were 12/12 `accepted`. Mixed: 60/60 HTTP 200, 54 `accepted`, 6 `runtime_error`, 63 clamp warnings — the errors and warnings being the deliberately-injected 256 KB under-allocations and over-allocation clamps, i.e. the load tester verifying the limit machinery rather than language failures.

**What we used / didn't use:**
Used the existing load tester as-is with the new payloads. Confirmed all ten languages pass under concurrency; interpreted the mixed-mode `runtime_error`/warning counts as expected behaviour, not regressions.
