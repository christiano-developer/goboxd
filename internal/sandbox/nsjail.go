// internal/sandbox/nsjail.go
package sandbox

import (
	"fmt"
	"os/exec"
	"strconv"
)

const NsjailPath = "/usr/local/bin/nsjail"

// Config holds the parameters for one nsjail invocation.
type Config struct {
	// WorkDir is the temp directory bind-mounted into the sandbox (rw).
	WorkDir string
	// WallTimeSecs is the max execution time in seconds.
	WallTimeSecs int
	// MemoryKB is the memory limit in kilobytes.
	MemoryKB int
	// MaxPIDs is the maximum number of processes/threads.
	MaxPIDs int
	// Command is the program to run inside the sandbox (e.g. ["python3", "solution.py"]).
	Command []string
}

// Build returns an exec.Cmd that runs Command inside an nsjail sandbox.
func Build(cfg Config) (*exec.Cmd, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("sandbox: command must not be empty")
	}

	memBytes := cfg.MemoryKB * 1024

	args := []string{
		// One-shot mode: run once then exit
		"--mode", "o",

		// Resource limits
		"--time_limit", strconv.Itoa(cfg.WallTimeSecs),
		"--rlimit_as", strconv.Itoa(memBytes),
		"--max_pids", strconv.Itoa(cfg.MaxPIDs),

		// Network: disabled
		"--disable_clone_newnet",

		// Filesystem: read-only system paths, rw work dir only
		"--bindmount_ro", "/usr",
		"--bindmount_ro", "/lib",
		"--bindmount_ro", "/lib64",
		"--bindmount_ro", "/etc/alternatives",

		// The working directory with submitted code (read-write)
		"--bindmount", cfg.WorkDir,
		"--cwd", cfg.WorkDir,

		// Log to stderr at WARNING level only (suppress noise)
		"--log_fd", "3",

		// Separator between nsjail args and the command to run
		"--",
	}

	// Append the actual command
	args = append(args, cfg.Command...)

	return exec.Command(NsjailPath, args...), nil
}
