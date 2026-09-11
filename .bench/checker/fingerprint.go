package checker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
)

// recordedSettings are the server settings every verdict carries, read back
// from the server rather than echoed from compose: the durability posture
// first, then what the postgres service pins.
var recordedSettings = []string{
	"synchronous_commit",
	"fsync",
	"full_page_writes",
	"autovacuum",
	"shared_buffers",
	"max_wal_size",
	"min_wal_size",
	"checkpoint_completion_target",
	"deadlock_timeout",
	"log_lock_waits",
	"checkpoint_timeout",
	"max_connections",
	"track_io_timing",
	"shared_preload_libraries",
}

// Fingerprint is the environment one run happened in. fingerprint.sh writes
// the library, image, host, and Docker facts before the run, since the
// checker's container can see none of them; the checker fills the Go and
// Postgres facts from its own binary and the server.
type Fingerprint struct {
	Execution    string            `json:"execution,omitempty"`
	Runtime      map[string]string `json:"runtime,omitempty"`
	BinarySha    string            `json:"binary_sha,omitempty"`
	LibrarySha   string            `json:"library_sha"`
	LibraryDirty bool              `json:"library_dirty"`
	GoVersion    string            `json:"go_version"`

	PostgresImage   string            `json:"postgres_image"`
	PostgresVersion string            `json:"postgres_version"`
	Settings        map[string]string `json:"settings"`

	Host   HostFingerprint   `json:"host"`
	Docker DockerFingerprint `json:"docker"`
}

// HostFingerprint is the machine the stack ran on.
type HostFingerprint struct {
	OS          string `json:"os"`
	CPU         string `json:"cpu"`
	Cores       int    `json:"cores"`
	MemoryBytes int64  `json:"memory_bytes"`
}

// DockerFingerprint is the engine the containers ran in -- on macOS a VM
// with its own CPU and memory budget, which is what the caps are cut from.
type DockerFingerprint struct {
	Cpus        int    `json:"cpus"`
	MemoryBytes int64  `json:"memory_bytes"`
	Version     string `json:"version"`
}

// ReadFingerprint decodes the file fingerprint.sh wrote and fills the Go
// version from the running binary.
func ReadFingerprint(path string) (*Fingerprint, error) {
	if path == "" {
		return nil, errors.New("path must not be empty")
	}

	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: %w -- run the scenario through just reliability-lab, which writes it", err)
	}
	fingerprint := &Fingerprint{}
	if err := json.Unmarshal(encoded, fingerprint); err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	if fingerprint.LibrarySha == "" {
		return nil, errors.New("fingerprint: library_sha is required")
	}
	if fingerprint.cpuCount() <= 0 {
		return nil, errors.New("fingerprint: execution CPU count must be positive")
	}
	fingerprint.GoVersion = runtime.Version()
	return fingerprint, nil
}

func (f *Fingerprint) cpuCount() int {
	if f.Execution == "native" {
		return f.Host.Cores
	}
	return f.Docker.Cpus
}
