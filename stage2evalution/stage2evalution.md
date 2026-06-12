# Payload Validation Report

**Endpoint:** `http://localhost:8080/run`  
**Date:** 2026-06-12 14:27:19  

## Summary

| Total | Passed | Failed | Skipped |
|------:|-------:|-------:|--------:|
| 13 | 10 | 3 | 0 |

## Results

| Status | Language | File | Expected | Got | Duration |
|--------|----------|------|----------|-----|----------|
| ✅ PASS | `php` | `accepted.json` | `accepted` | `accepted` | 118ms |
| ✅ PASS | `php` | `wrong_output.json` | `wrong_output` | `wrong_output` | 14ms |
| ✅ PASS | `php` | `runtime_error.json` | `runtime_error` | `runtime_error` | 14ms |
| ✅ PASS | `php` | `time_exceeded.json` | `time_exceeded` | `time_exceeded` | 9.011s |
| ❌ FAIL | `kotlin` | `accepted.json` | `accepted` | `runtime_error` | 1.671s |
| ❌ FAIL | `kotlin` | `wrong_output.json` | `wrong_output` | `runtime_error` | 1.502s |
| ✅ PASS | `kotlin` | `runtime_error.json` | `runtime_error` | `runtime_error` | 1.435s |
| ❌ FAIL | `kotlin` | `time_exceeded.json` | `time_exceeded` | `runtime_error` | 1.388s |
| ✅ PASS | `kotlin` | `build_failed.json` | `build_failed` | `build_failed` | 1.071s |
| ✅ PASS | `lisp` | `accepted.json` | `accepted` | `accepted` | 50ms |
| ✅ PASS | `lisp` | `wrong_output.json` | `wrong_output` | `wrong_output` | 7ms |
| ✅ PASS | `lisp` | `runtime_error.json` | `runtime_error` | `runtime_error` | 18ms |
| ✅ PASS | `lisp` | `time_exceeded.json` | `time_exceeded` | `time_exceeded` | 10.009s |

## Failed Cases

### `payloads/kotlin/accepted.json`

- **Expected:** `accepted`
- **Got:** `runtime_error`

### `payloads/kotlin/wrong_output.json`

- **Expected:** `wrong_output`
- **Got:** `runtime_error`

### `payloads/kotlin/time_exceeded.json`

- **Expected:** `time_exceeded`
- **Got:** `runtime_error`
