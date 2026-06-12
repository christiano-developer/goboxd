# Load-test run archive

One CSV + its plots per benchmark run, named by workload + concurrency + a
timestamp so sequential runs never collide. The canonical "latest" copy lives at
`../results.csv` with `../breaking-point.png` / `../latency.png`; this folder is
the history for comparing runs.

## TL;DR — run the load test

With goboxd already up (see below to (re)start it), from the **repo root**:

```bash
# everything in one shot: test -> timestamped CSV + plots in runs/ + latest copies
go run ./scripts/memhog \
  -source docs/loadtest/MemoryHog.java \
  -python /tmp/plotenv/bin/python
```

Or the wrapper, which sets the same defaults and is env-overridable
(`RATES=… DURATION=… PYTHON=…`):

```bash
docs/loadtest/load-test.sh
```

`-python` must point at a Python that has matplotlib (one-time setup:
`python3.12 -m venv /tmp/plotenv && /tmp/plotenv/bin/pip install -q matplotlib`).
Add `-plot=false` to skip plots. The run is labelled from the server's live
`CONCURRENCY_LIMIT`, so no flags are needed to tag it.

## How to run a new instance (varying `CONCURRENCY_LIMIT`)

`CONCURRENCY_LIMIT` is an **environment variable** in `docker-compose.yml`, so a
change only needs a container **recreate** — no image rebuild. (The java
`-Xmx384m` heap config is baked into the image; only rebuild if you edit
`configs/languages/languages.yaml`.)

```bash
# 1. Set the limit in docker-compose.yml (service: goboxd)
#      environment:
#        - CONCURRENCY_LIMIT=100

# 2. Recreate the container so the new env takes effect
docker compose up -d --force-recreate goboxd

# 3. Wait until ready, and confirm the limit that actually loaded
until curl -sf http://localhost:8080/readyz >/dev/null; do sleep 1; done
docker logs goboxd 2>&1 | grep "concurrency pool" | tail -1
#   -> ...,"concurrency_limit":100,...

# 4. (one-time) a Python venv with matplotlib for the plots
python3.12 -m venv /tmp/plotenv && /tmp/plotenv/bin/pip install -q matplotlib

# 5. Run the benchmark — auto-labels the run c100 from /info, writes a
#    timestamped CSV + per-run plots here, and refreshes ../results.csv
go run ./scripts/memhog \
  -source docs/loadtest/MemoryHog.java \
  -python /tmp/plotenv/bin/python
```

Produces (example): `memhog_c100_2vcpu-2gb_20060102-150405.csv` plus
`..._breaking-point.png`, `..._latency.png`, `..._wasted-work.png`.

### Useful overrides

| Flag | Default | Purpose |
|---|---|---|
| `-rates` | `1,2,3,5,10,25,50,75,100,150,200,300,400` | the RPS ladder |
| `-duration` | `30s` | hold time per step |
| `-timeout` | `10s` | per-request SLA (slower = failed) |
| `-stop-after-fail` | `3` | steps to run past the first failure |
| `-tag` | `2vcpu-2gb` | label suffix (change if you resize the box) |
| `-label` | auto `c<N>` from /info | override the run label entirely |
| `-plot=false` | (plots on) | skip plotting |

If you only changed concurrency, **do not** `make build`. If you changed
`languages.yaml`, run `make build` before step 2.

## CSV schema

Columns 1–11 are the challenge schema; 12–16 are extra work-accounting signals:

```
target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms,server_completed,wasted,wasted_pct,timeouts,shed_503
```

- `server_completed` — jobs the server actually finished this step (`/info` `jobs_total` delta).
- `wasted` — finished **after** the client had already timed out (abandoned work; the running phase isn't cancelled on client disconnect).
- `wasted_pct` — `wasted / server_completed`.
- `timeouts`, `shed_503` — failure-mode breakdown.

## Runs

| File | Concurrency | Limits | Breaking point | Notes |
|---|---|---|---|---|
| `memhog_2vcpu-2gb_xmx384_30s_2026-06-12.csv` | 2 | 2 vCPU / 2 GB | 3 rps | first run (pre-rename; no plots / extra cols) |
| `memhog_c8_2vcpu-2gb_20260612-172224.csv` | **8** | 2 vCPU / 2 GB | **5 rps** | **best config tested** — see below |
| `memhog_c16_2vcpu-2gb_20260612-173319.csv` | 16 | 2 vCPU / 2 GB | 5 rps | same breaking point as c8 but worse past it (more thrash) |
| `memhog_c50_2vcpu-2gb_<ts>.csv` | 50 | 2 vCPU / 2 GB | 5 rps | instant saturation; heavy congestion collapse |

## Best result — `CONCURRENCY_LIMIT=8`

`memhog_c8_2vcpu-2gb_20260612-172224.csv` is the best configuration tested
(plots: `..._breaking-point.png`, `..._latency.png`, `..._wasted-work.png`).

| offered rps | throughput/s | error % | p50 ms | server_completed | wasted | wasted % |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 0.96 | 0.0 | 1291 | 29 | 0 | 0 |
| 2 | 1.92 | 0.0 | 1275 | 59 | 0 | 0 |
| 3 | 2.88 | 0.0 | 1255 | 89 | 0 | 0 |
| **5** | 1.66 | **55.7** | 10000 | 119 | 53 | 44.5 |
| 10 | 0.73 | 90.3 | 10000 | 110 | 81 | 73.6 |
| 25 | 0.60 | 96.8 | 10000 | 112 | 88 | 78.6 |
| 50 | 0.55 | 98.5 | 10000 | 112 | 90 | 80.4 |

**Breaking point = 5 rps** (first offered rate with a failed request).

### Why this is the best config

The bottleneck for the MemoryHog workload is **CPU/wall-time, not memory**: each
request costs ~1.3 s (JVM compile + run + a fixed 1 s `sleep`), and the box has
only **2 vCPU**. Capacity ≈ `2 CPUs ÷ CPU-seconds-per-request`. Because the 1 s
sleep is *idle* (not CPU), a handful of concurrent jobs can overlap their sleeps
and lift throughput — but only up to a point.

- **c8 hits that sweet spot.** Enough slots to overlap the sleeps (clean through
  3 rps at 2.88/s), without over-subscribing 2 cores.
- **More concurrency is worse, not better.** c16 and c50 break at the *same*
  5 rps but complete *less* real work past the knee (c8 finishes **119** jobs at
  5 rps vs c16's 84) — 16+ JVMs fighting over 2 cores thrash on CPU and memory.
  This is **congestion collapse**: adding load/slots reduces goodput.
- **Concurrency is not the lever.** c8, c16, c50 all break at 5 rps → the limit
  is the 2 vCPU budget, not the slot count. Raising `CONCURRENCY_LIMIT` past ~8
  only degrades behaviour.

### How it fails (and why that's acceptable)

Every failure is a **client-side 10 s timeout**, not a crash: `shed_503 ≈ 0`,
`OOMKilled=false`. The server keeps accepting and queuing requests; past capacity
the queue wait exceeds the 10 s SLA and clients give up. The `wasted` column
shows the cost of that — 44–80% of the work the server *completes* past the knee
is delivered after the client already left (the run phase isn't cancelled on
disconnect). Below the breaking point, 0% waste and 0% errors.

**Takeaway:** at a fixed 2 vCPU / 2 GB this service sustains ~3–5 rps of a heavy
memory workload, peaking at `CONCURRENCY_LIMIT=8`. To go higher you'd add CPU,
cut per-request cost (warm JVM / AppCDS), or stop wasting CPU on abandoned work
(deadline-aware admission, cancel-on-disconnect) — not raise concurrency.
