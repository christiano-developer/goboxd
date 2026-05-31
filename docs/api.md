# API Reference (Stage 1)

This document describes the HTTP endpoints, payload definitions, and validation constraints implemented for **Stage 1** of the goboxd service.

---

## Endpoints

| Method | Path | Description | Access |
|---|---|---|---|
| `GET` | `/healthz` | Lightweight liveness probe | Public |
| `POST` | `/run` | Compiles and executes untrusted code in an isolated sandbox | Public |

---

## 1. GET /healthz
Checks if the HTTP server is up and responsive.

### Response
* **Status Code:** `200 OK`
* **Content-Type:** `application/json`
```json
{
  "status": "ok"
}
```

---

## 2. POST /run
Submits code to be written, compiled (optional), and run under `nsjail` isolation against test cases.

### Request Payload (`RunRequest`)
```json
{
  "language": "c",
  "source": "#include <stdio.h>\nint main() { printf(\"Hello!\"); return 0; }",
  "source_filename": "solution.c",
  "artifact_filename": "solution",
  "build": {
    "limits": { "wall_time_s": 15, "memory_kb": 1048576, "max_processes": 100 },
    "flags": ["-O2", "-std=c11"]
  },
  "run": {
    "limits": { "wall_time_s": 10, "memory_kb": 128000, "max_processes": 32 },
    "flags": []
  },
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "Hello!"
    }
  ]
}
```

#### Fields Description:
* `language` *(string, Required)*: The identifier matching a registered language runtime (`py3` or `c` for Stage 1).
* `source` *(string, Required)*: UTF-8 source code to run. Must be non-empty and ≤ 256 KiB.
* `source_filename` *(string, Optional)*: Custom filename. Must be a single path component (no directory separators `/` or `\`) and cannot start with `.`.
* `artifact_filename` *(string, Optional)*: Custom compiled output filename. Must be a single path component and cannot start with `.`.
* `build` / `run` *(object, Optional)*: Limits overrides and compiler/interpreter flags.
  * `limits` *(object, Optional)*: Wall time, memory, or process limit bounds.
  * `flags` *(array of strings, Optional)*: Custom compiler flags. Checked against a configuration allow-list.
* `tests` *(array of objects, Required)*: Non-empty list of test cases (capped at 50 cases in Stage 1).

---

### Response Payload (`RunResponse`)
The server always returns HTTP `200 OK` even if user code fails to compile or run, detailing the failure inside the JSON response.

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 74
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello!",
      "stderr": "",
      "duration_ms": 2
    }
  ]
}
```

#### Status Vocabulary

| Scope | Value | Description |
|---|---|---|
| `build.status` | `ok` | Compilation completed successfully. |
| | `failed` | Compilation returned a non-zero exit code. |
| | `internal_error` | Server error setting up compilation. |
| `tests[].status` | `accepted` | Test stdout matched expected value exactly (whitespace ignored/normalized). |
| | `wrong_output` | Executed successfully but stdout did not match. |
| | `output_whitespace_mismatch` | Output matched expected value except for white space differences. |
| | `time_exceeded` | Process killed because execution exceeded the wall-time limit. |
| | `memory_exceeded` | Process killed because execution exceeded the memory limit. |
| | `runtime_error` | Process exited with a non-zero code or signal. |
| | `not_executed` | Test skipped (e.g. because compilation failed). |
| | `internal_error` | Sandbox environment failed to initiate. |
| **Top-Level `status`** | Same as above | Returns `accepted` only if all tests succeeded. Otherwise, returns the status of the first failing step/test. If the build step fails, returns `build_failed`. |

---

### Error Payloads (HTTP 400 Bad Request)
Returned when payload validation fails prior to execution.

```json
{
  "error": {
    "code": "disallowed_flag",
    "message": "flag \"-Ofast\" is not allowed for this language"
  }
}
```

#### Error Code Types:
* `invalid_json`: Bad request payload JSON format.
* `missing_language`: Request did not specify a `language` identifier.
* `unknown_language`: Specified language is not supported.
* `invalid_source`: Code size exceeds 256 KiB or is empty.
* `invalid_tests`: Missing tests array or too many tests (capped at 50).
* `invalid_filename`: Filename contains path separators or dot prefixes.
* `disallowed_flag`: Build flag not matching the runtime's configuration allow-list.
