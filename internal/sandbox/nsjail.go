// internal/sandbox/nsjail.go
// Christiano Fernandes
// 31 May 26
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

	// nsjail expects --rlimit_as in MB, not bytes or KB
	memMB := (cfg.MemoryKB + 1023) / 1024
	if cfg.MemoryKB > 0 && memMB == 0 {
		memMB = 1
	}

	args := []string{
		// One-shot mode: run once then exit
		"--mode", "o",

		// Resource limits
		"--time_limit", strconv.Itoa(cfg.WallTimeSecs),
		"--rlimit_as", strconv.Itoa(memMB),
		"--rlimit_nproc", strconv.Itoa(cfg.MaxPIDs),

		// Network: disabled
		"--disable_clone_newnet",

		// chroot to the host root (read-only)
		"--chroot", "/",

		// Writable temp space for compilers/interpreters
		"--tmpfsmount", "/tmp",

		// The working directory with submitted code (read-write)
		"--bindmount", cfg.WorkDir,
		"--cwd", cfg.WorkDir,

		// Pass clean standard PATH for compiler sub-commands (like ld and as)
		"--env", "PATH=/usr/bin:/bin",

		// Log only fatal errors from nsjail to stderr (default FD 2)
		"--really_quiet",

		// Separator between nsjail args and the command to run
		"--",
	}

	// Append the actual command
	args = append(args, cfg.Command...)

	return exec.Command(NsjailPath, args...), nil
}
