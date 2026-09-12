package observer

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/allegedlyreliable/sqlstreams/.bench/record"
)

// HostObserver samples local role CPU; it requires no access to the database host.
type HostObserver struct {
	writer   *record.Writer
	previous map[int]float64
	at       time.Time
}

func NewHostObserver(directory string) (*HostObserver, error) {
	writer, err := record.NewWriter(directory, "host", record.FileKindContainer)
	if err != nil {
		return nil, err
	}
	return &HostObserver{writer: writer}, nil
}

func (h *HostObserver) Sample(ctx context.Context, processes map[string]int) error {
	ctx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	now := time.Now()
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,time=").Output()
	if err != nil {
		return err
	}
	current := map[int]float64{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return err
		}
		seconds, err := processSeconds(fields[1])
		if err != nil {
			return err
		}
		current[pid] = seconds
	}
	for name, pid := range processes {
		if name != "producer" && !strings.HasPrefix(name, "consumer") {
			continue
		}
		seconds, present := current[pid]
		previous, observed := h.previous[pid]
		if !present || !observed {
			continue
		}
		service := "consumer"
		if name == "producer" {
			service = name
		}
		// Local processes have no CPU quota; the fingerprint supplies host capacity.
		if err := h.writer.Write(record.ContainerRecord{At: now, Name: name, Service: service,
			CpuPercent: max(0, seconds-previous) / now.Sub(h.at).Seconds() * 100}); err != nil {
			return err
		}
	}
	h.previous, h.at = current, now
	return nil
}

func (h *HostObserver) Close() error { return h.writer.Close() }

func processSeconds(value string) (float64, error) {
	days := 0.0
	if before, after, ok := strings.Cut(value, "-"); ok {
		parsed, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0, err
		}
		days = parsed
		value = after
	}
	total := 0.0
	for _, part := range strings.Split(value, ":") {
		seconds, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, err
		}
		total = total*60 + seconds
	}
	return days*86400 + total, nil
}
