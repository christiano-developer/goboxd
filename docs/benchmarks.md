# goboxd — Concurrency Benchmarks

This document records the performance benchmarks of the **goboxd** service under concurrent loads.

## Test Environment
*   **Host OS**: macOS (Darwin 25.4.0)
*   **Hardware**: Apple M2 MacBook Air (8 CPU cores, 8 GB RAM)
*   **Virtualization**: Docker Desktop (running Linux container environment, unprivileged mode)
*   **Target Payload**: Trivial Python 3 execution (`py3`, "Hello World" print)

---

## Baseline Benchmark Results (Python 3)

The following table summarizes the requests/sec throughput and latency percentiles ($p_{50}$, $p_{95}$, $p_{99}$) for `POST /run` under different concurrent client configurations.

| Concurrent Clients | Total Requests | Throughput (req/sec) | Average Latency | $p_{50}$ (Median) | $p_{95}$ | $p_{99}$ |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **1** | 100 | 94.61 | 10.52ms | 8.42ms | 15.76ms | 144.36ms |
| **10** | 100 | 237.81 | 40.90ms | 33.42ms | 106.24ms | 132.43ms |
| **50** | 500 | 361.35 | 132.38ms | 122.28ms | 203.54ms | 263.01ms |
| **100** | 1000 | 407.66 | 234.62ms | 231.64ms | 276.41ms | 345.31ms |

---

## Mixed Payload and Limit Clamping Verification

To verify that the queue scheduler, priority routing, and resource limit clamping function correctly under realistic workloads, we ran a mixed payload benchmark:
*   **Configuration:** 10 concurrent clients sending 100 total requests.
*   **Workload:** Randomized mix of all 7 supported languages (Python, C, C++, Java, Bash, JavaScript, and Verilog) with randomized over-allocated and under-allocated resource limits.
*   **Outcome:** 100% of requests processed successfully (HTTP 200).
*   **Throughput:** **45.33 requests/sec**.
*   **Latency Profile:**
    *   **Average:** 203.95ms
    *   **p50 (Median):** 105.53ms
    *   **p95:** 792.55ms
    *   **p99:** 937.16ms
*   **Clamping Assertion:** **110 warnings** were returned in the response payloads, confirming that both under-allocation floors (e.g. JVM/Node 1 GB minimums) and load-adaptive upper caps were correctly computed and active.
