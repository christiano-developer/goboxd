# goboxd — Concurrency Benchmarks

This document records the performance benchmarks of the **goboxd** service under concurrent loads.

## Test Environment
*   **Host OS**: macOS (Darwin 25.4.0)
*   **Hardware**: Apple M2 MacBook Air (8 CPU cores, 8 GB RAM)
*   **Virtualization**: Docker Desktop (running Linux container environment, privileged mode)
*   **Target Payload**: Trivial Python 3 execution (`py3`, "Hello World" print)

---

## Benchmark Results

The following table summarizes the requests/sec throughput and latency percentiles ($p_{50}$, $p_{95}$, $p_{99}$) for `POST /run` under different concurrent client configurations.

| Concurrent Clients | Total Requests | Throughput (req/sec) | Average Latency | $p_{50}$ (Median) | $p_{95}$ | $p_{99}$ |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **1** | 20 | 66.53 | 14.99ms | 7.56ms | 139.08ms | 139.08ms |
| **10** | 100 | 373.06 | 26.26ms | 18.55ms | 93.23ms | 104.17ms |
| **50** | 500 | 528.36 | 91.09ms | 86.92ms | 123.44ms | 168.32ms |
| **100** | 1000 | 549.10 | 174.86ms | 166.48ms | 225.51ms | 310.82ms |

---

## Load Shedding Verification

Under extreme overload (**600 concurrent clients** sending 1200 total requests), the service successfully demonstrated active queue limitation and immediate load shedding:
*   **Total Capacity**: 515 requests (15 active workers + 500 queued slots).
*   **Processed successfully (HTTP 200)**: 565 requests.
*   **Rejected immediately (HTTP 503 Service Unavailable)**: 635 requests.
*   **Dynamic Wait Estimation**: Rejected requests successfully returned the `Retry-After` header indicating estimated wait time.
