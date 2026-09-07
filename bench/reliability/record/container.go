package record

import "time"

// ContainerRecord is one docker stats reading stats.sh wrote from the host:
// a container's CPU as a percent of one core, the compose service it runs,
// and the cores compose capped it at -- 0 when uncapped.
type ContainerRecord struct {
	At         time.Time `json:"at"`
	Name       string    `json:"name"`
	Service    string    `json:"service"`
	CpuPercent float64   `json:"cpu_percent"`
	Cpus       float64   `json:"cpus"`
}
