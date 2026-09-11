package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/observer"
	runnerdatastore "github.com/agentstax/sqlstreams/.bench/runner/datastore"
	"github.com/agentstax/sqlstreams/.bench/scenario"
)

// Manager owns complete runs; each child continues to execute one Runner role.
type Manager struct {
	declared    *scenario.Scenario
	cfg         *ManagerConfig
	binary      string
	directory   string
	name        string
	environment []string
	processes   map[string]*process
}

func NewManager(declared *scenario.Scenario, cfg *ManagerConfig) (*Manager, error) {
	if declared == nil {
		return nil, errors.New("declared must not be nil")
	}
	if cfg == nil {
		cfg = &ManagerConfig{}
	}
	if err := cfg.WithDefaults().Validate(); err != nil {
		return nil, err
	}
	if err := declared.Validate(); err != nil {
		return nil, err
	}
	if err := declared.Scaled(cfg.TimeScale).Validate(); err != nil {
		return nil, err
	}
	if filepath.Base(declared.Name) != declared.Name || declared.Name == "." || declared.Name == ".." {
		return nil, errors.New("scenario name must be one directory name")
	}
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	results, err := filepath.Abs(cfg.ResultsDir)
	if err != nil {
		return nil, err
	}
	cfg.ResultsDir = results
	return &Manager{declared: declared, cfg: cfg, binary: binary}, nil
}

// Run returns the worst repetition's verdict. Failure outranks unknown;
// an orchestration or cleanup error ends the run with an error instead.
func (m *Manager) Run(ctx context.Context) (int, error) {
	m.name = "reliability_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	m.environment = nil
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "SCENARIO_FILE=") && (m.cfg.Execution == "native" || !strings.HasPrefix(value, "POSTGRES_")) {
			m.environment = append(m.environment, value)
		}
	}
	if m.cfg.Execution == "compose" {
		m.environment = append(m.environment, "COMPOSE_PROJECT_NAME="+m.name, "LAB_RESULTS_DIR="+m.cfg.ResultsDir)
		if err := m.compose(ctx, "--profile", "checker", "build"); err != nil {
			return 0, err
		}
	}
	worst := 0
	for repetition := 1; repetition <= m.cfg.Repetitions; repetition++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		fmt.Printf("repetition %d/%d (%s)\n", repetition, m.cfg.Repetitions, m.cfg.Execution)
		code, err := m.run(ctx, repetition)
		if err != nil {
			return 0, err
		}
		if code == 3 || code == 1 && worst != 3 || code == 2 && worst == 0 {
			worst = code
		}
	}
	return worst, nil
}

func (m *Manager) run(ctx context.Context, repetition int) (code int, err error) {
	m.directory = filepath.Join(m.cfg.ResultsDir, m.declared.Name, fmt.Sprintf("%s-%d", m.name, repetition))
	m.processes = map[string]*process{}
	if err = os.MkdirAll(filepath.Join(m.directory, "records"), 0o755); err != nil {
		return 0, err
	}
	fmt.Printf("run %s; results %s\n", filepath.Base(m.directory), m.directory)
	declaration, err := json.Marshal(m.declared)
	if err != nil {
		return 0, err
	}
	if err = os.WriteFile(filepath.Join(m.directory, "declaration.json"), declaration, 0o644); err != nil {
		return 0, err
	}
	var host *observer.HostObserver
	if m.cfg.Execution == "native" {
		connection, err := common.NewConnection(ctx, 1)
		if err != nil {
			return 0, err
		}
		ds, err := runnerdatastore.NewRunnerDatastore(connection.Pool)
		if err == nil {
			err = ds.CheckRunDatabase(ctx)
		}
		connection.Close()
		if err != nil {
			return 0, err
		}
		host, err = observer.NewHostObserver(m.directory)
		if err != nil {
			return 0, err
		}
		defer func() { err = errors.Join(err, host.Close()) }()
	} else {
		defer func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 2*time.Minute)
			defer stop()
			err = errors.Join(err, m.compose(cleanupCtx, "down", "-v", "--remove-orphans"))
		}()
		if err = m.compose(ctx, "up", "--detach", "--wait", "postgres"); err != nil {
			return 0, err
		}
		if err = m.compose(ctx, "exec", "-T", "postgres", "psql", "-U", "lab", "-d", "lab", "-v", "ON_ERROR_STOP=1", "-c", "ALTER DATABASE lab SET synchronous_commit = "+m.cfg.SynchronousCommit); err != nil {
			return 0, err
		}
	}
	fingerprint := exec.CommandContext(ctx, "bash", "fingerprint.sh", m.cfg.Execution)
	fingerprint.Env = m.environment
	encoded, err := fingerprint.Output()
	if err != nil {
		return 0, err
	}
	if err = os.WriteFile(filepath.Join(m.directory, "fingerprint.json"), encoded, 0o644); err != nil {
		return 0, err
	}
	defer func() {
		// Signal every role before waiting so their shutdown budgets overlap.
		for _, running := range m.processes {
			err = errors.Join(err, running.signal())
		}
		for _, running := range m.processes {
			err = errors.Join(err, running.stop(), running.file.Close())
		}
	}()
	if m.cfg.Execution == "compose" {
		stats := m.command("bash", "stats.sh")
		stats.Stderr = os.Stderr
		if err = m.start("host-observer", stats, "host.container.jsonl"); err != nil {
			return 0, err
		}
	}
	for replica := 1; replica <= m.cfg.Replicas; replica++ {
		name := "consumer"
		if replica > 1 {
			name += "-" + strconv.Itoa(replica)
		}
		if err = m.startRole("consumer", name); err != nil {
			return 0, err
		}
	}
	if err = m.startRole("observer", "observer"); err != nil {
		return 0, err
	}
	if err = common.WaitUntil(ctx, time.Now().Add(3*time.Second)); err != nil {
		return 0, err
	}
	if err = m.startRole("producer", "producer"); err != nil {
		return 0, err
	}
	if code, err = m.wait(ctx, "producer", host); err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, errors.New("producer exited before completing the scenario; inspect producer.log")
	}
	if err = m.stopRole(ctx, "observer"); err != nil {
		return 0, err
	}
	if m.cfg.Execution == "compose" {
		if err = m.processes["host-observer"].stop(); err != nil {
			return 0, err
		}
	}
	if err = m.startRole("checker", "checker"); err != nil {
		return 0, err
	}
	code, err = m.wait(ctx, "checker", nil)
	if err != nil {
		return 0, err
	}
	fmt.Printf("results %s; checker exit %d\n", m.directory, code)
	return code, err
}

func (m *Manager) startRole(role string, name string) error {
	directory, records, results := m.directory, filepath.Join(m.directory, "records"), m.cfg.ResultsDir
	if m.cfg.Execution == "compose" {
		directory = filepath.Join("/results", m.declared.Name, filepath.Base(m.directory))
		records, results = "/records", "/results"
	}
	arguments := []string{"-role", role, "-name", name, "-scenario-file", filepath.Join(directory, "declaration.json"),
		"-time-scale", strconv.FormatFloat(m.cfg.TimeScale, 'g', -1, 64), "-record-dir", records}
	if role == "checker" {
		arguments = append(arguments, "-run-dir", directory, "-results-dir", results,
			"-fingerprint-file", filepath.Join(directory, "fingerprint.json"), "-stats-file", filepath.Join(directory, "host.container.jsonl"),
			"-drain-budget", m.cfg.DrainBudget.String())
	}
	command := m.command(m.binary, arguments...)
	if m.cfg.Execution == "compose" {
		command = m.command("docker", append([]string{"compose", "run", "--rm", "--no-deps", "--name", m.name + "_" + name, role}, arguments...)...)
	}
	return m.start(name, command, name+".log")
}

func (m *Manager) start(name string, command *exec.Cmd, file string) error {
	started, err := newProcess(command, filepath.Join(m.directory, file))
	if err != nil {
		return err
	}
	m.processes[name] = started
	return nil
}

func (m *Manager) stopRole(ctx context.Context, name string) error {
	if m.cfg.Execution == "compose" {
		command := exec.CommandContext(ctx, "docker", "stop", "--time", "90", m.name+"_"+name)
		if err := command.Run(); err != nil {
			return err
		}
		m.processes[name].stopRequested = true
	}
	return m.processes[name].stop()
}

func (m *Manager) wait(ctx context.Context, name string, host *observer.HostObserver) (int, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-m.processes[name].done:
			var exited *exec.ExitError
			if errors.As(m.processes[name].err, &exited) {
				if exited.ExitCode() < 0 || exited.ExitCode() > 3 {
					return 0, exited
				}
				return exited.ExitCode(), nil
			}
			return 0, m.processes[name].err
		case <-ticker.C:
			for role, running := range m.processes {
				if !strings.HasPrefix(role, "consumer") && !(name == "producer" && (role == "observer" || role == "host-observer")) {
					continue
				}
				select {
				case <-running.done:
					return 0, fmt.Errorf("%s exited before %s completed; inspect its log", role, name)
				default:
				}
			}
			if host != nil {
				processes := map[string]int{}
				for role, running := range m.processes {
					processes[role] = running.command.Process.Pid
				}
				if err := host.Sample(ctx, processes); err != nil {
					return 0, err
				}
			}
		}
	}
}

func (m *Manager) command(name string, arguments ...string) *exec.Cmd {
	command := exec.Command(name, arguments...)
	command.Env = m.environment
	return command
}

func (m *Manager) compose(ctx context.Context, arguments ...string) error {
	command := exec.CommandContext(ctx, "docker", append([]string{"compose"}, arguments...)...)
	command.Env = m.environment
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}
