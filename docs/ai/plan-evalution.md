# docs/ai/plan-evolution.md
# Plan Evolution Log (Stage 1)

## 2026-05-31 · Pivot from custom chroot directories to read-only host-root chroot (`--chroot /`)

**What we thought we'd do:**
Isolate executions inside a minimal custom chroot directory containing only the compiled user executable/source files.

**What we actually did:**
Switched to `--chroot /` (mounting the host root read-only in nsjail), combined with read-only binds of system runtime paths (`/lib`, `/usr`) and a writable `tmpfs` mounted at `/tmp`.

**Why it changed:**
Running under a custom minimal chroot broke interpreters (Python) and compilers (GCC) which failed with "shared library not found" and "header not found" errors (e.g., missing `libpython3.11.so.1.0` or `stdio.h`). Mounting the host root read-only securely resolved these dependencies while keeping the user code containerized and isolated.

---

## 2026-05-31 · Trimming Stage 1 scope to meet the June 1 EOD deadline

**What we thought we'd do:**
Implement all features (concurrency pool, security boundary verification, load testing, all 7 languages, and `/readyz`/`/info` endpoints) in a single massive development push.

**What we actually did:**
Separated Stage 1 must-ship features from subsequent stages. Focused strictly on `POST /run` for Python and C, basic `/healthz`, isolation via nsjail, Docker compilation stages, and initial unit/integration tests.

**Why it changed:**
The competition timeline explicitly splits Stage 1 (due June 1 EOD) from Stage 2/3. To ensure a solid, well-tested prototype was delivered on time, we trimmed unnecessary scope and focused on core stability.

---

## 2026-05-31 · Health endpoint response payload

**What we thought we'd do:**
Implement `GET /healthz` returning a simple plain text string `"ok"` with status code 200.

**What we actually did:**
Updated `GET /healthz` to return a structured JSON object `{"status":"ok"}`.

**Why it changed:**
To ensure exact compliance with the JSON API contract specified in the official competition brief.
