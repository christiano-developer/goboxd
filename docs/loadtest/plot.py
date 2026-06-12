#!/usr/bin/env python3
"""Plot breaking-point.png and latency.png from results.csv.

Usage: python3 docs/loadtest/plot.py [results.csv] [--breaking-point RPS]
"""
import csv
import sys

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt


def load(path):
    with open(path) as f:
        return list(csv.DictReader(f))


def find_breaking_point(rows):
    """First offered RPS with a failure (error_pct > 0)."""
    for r in rows:
        if float(r["error_pct"]) > 0:
            return float(r["target_rps"])
    return None


def main():
    path = "docs/loadtest/results.csv"
    bp = None
    outdir = "docs/loadtest"
    prefix = ""          # "" -> breaking-point.png ; "foo" -> foo_breaking-point.png
    title_suffix = "2 vCPU / 2 GB"
    args = sys.argv[1:]
    i = 0
    while i < len(args):
        if args[i] == "--breaking-point":
            bp = float(args[i + 1]); i += 2
        elif args[i] == "--outdir":
            outdir = args[i + 1]; i += 2
        elif args[i] == "--prefix":
            prefix = args[i + 1]; i += 2
        elif args[i] == "--title":
            title_suffix = args[i + 1]; i += 2
        else:
            path = args[i]; i += 1

    pre = (prefix + "_") if prefix else ""
    bp_png = f"{outdir}/{pre}breaking-point.png"
    lat_png = f"{outdir}/{pre}latency.png"
    waste_png = f"{outdir}/{pre}wasted-work.png"

    rows = load(path)
    rps = [float(r["target_rps"]) for r in rows]
    if bp is None:
        bp = find_breaking_point(rows)

    # ---- breaking-point.png: offered RPS vs error rate ----
    plt.figure(figsize=(8, 5))
    plt.plot(rps, [float(r["error_pct"]) for r in rows], marker="o", color="#c0392b")
    if bp is not None:
        plt.axvline(bp, color="#2c3e50", linestyle="--", linewidth=1.5,
                    label=f"breaking point = {bp:g} rps")
        plt.legend()
    plt.xlabel("Offered RPS")
    plt.ylabel("Error rate (%)")
    plt.title(f"Breaking point — MemoryHog @ {title_suffix}")
    plt.grid(True, alpha=0.3)
    plt.savefig(bp_png, dpi=150, bbox_inches="tight")

    # ---- latency.png: offered RPS vs p50/p95/p99 ----
    plt.figure(figsize=(8, 5))
    for key, lbl, color in [("p50_ms", "p50", "#27ae60"),
                            ("p95_ms", "p95", "#f39c12"),
                            ("p99_ms", "p99", "#c0392b")]:
        plt.plot(rps, [float(r[key]) for r in rows], marker="o", label=lbl, color=color)
    if bp is not None:
        plt.axvline(bp, color="#2c3e50", linestyle="--", linewidth=1.5,
                    label=f"breaking point = {bp:g} rps")
    plt.xlabel("Offered RPS")
    plt.ylabel("Latency (ms)")
    plt.title(f"RPS vs latency — MemoryHog @ {title_suffix}")
    plt.legend()
    plt.grid(True, alpha=0.3)
    plt.savefig(lat_png, dpi=150, bbox_inches="tight")

    # ---- wasted-work.png: delivered (goodput) vs wasted server work ----
    # Only plotted if the CSV carries the extra accounting columns.
    if rows and "server_completed" in rows[0]:
        plt.figure(figsize=(8, 5))
        plt.plot(rps, [float(r["success"]) for r in rows], marker="o",
                 label="delivered (within SLA)", color="#27ae60")
        plt.plot(rps, [float(r["wasted"]) for r in rows], marker="o",
                 label="wasted (finished after client gave up)", color="#c0392b")
        plt.plot(rps, [float(r["server_completed"]) for r in rows], marker="o",
                 label="server total completed", color="#2c3e50", linestyle="--")
        if bp is not None:
            plt.axvline(bp, color="#7f8c8d", linestyle=":", linewidth=1.2)
        plt.xlabel("Offered RPS")
        plt.ylabel("Jobs per step")
        plt.title(f"Delivered vs wasted server work — MemoryHog @ {title_suffix}")
        plt.legend()
        plt.grid(True, alpha=0.3)
        plt.savefig(waste_png, dpi=150, bbox_inches="tight")
        print(f"wrote {waste_png}")

    print(f"breaking point: {bp} rps")
    print(f"wrote {bp_png} and {lat_png}")


if __name__ == "__main__":
    main()
