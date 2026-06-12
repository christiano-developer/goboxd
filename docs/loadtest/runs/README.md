# Load-test run archive

One CSV per benchmark run, named by workload + container limits + key params + date.
The canonical "latest" copy stays at `../results.csv` (what the graphs are built from);
this folder keeps the history so runs can be compared.

Schema (all files): `target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms`

| File | Workload | Limits | Notes |
|---|---|---|---|
| `memhog_2vcpu-2gb_xmx384_30s_2026-06-12.csv` | java MemoryHog (150 MB) | 2 vCPU / 2 GB, CONCURRENCY_LIMIT=2 | java run -Xmx384m; 30 s/step, 10 s timeout. Breaking point = 3 rps. |
