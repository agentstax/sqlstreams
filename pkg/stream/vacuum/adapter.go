package vacuum

func toVacuumMetadata(cfg *VacuumConfig) *vacuumMetadata {
	return &vacuumMetadata{
		PollRate:      cfg.PollRate,
		VacuumTimeout: cfg.VacuumTimeout,
	}
}
