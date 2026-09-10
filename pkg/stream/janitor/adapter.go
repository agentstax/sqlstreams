package janitor

func toJanitorMetadata(cfg *JanitorConfig) *janitorMetadata {
	return &janitorMetadata{
		PollRate:                cfg.PollRate,
		SweepBatchSize:          cfg.SweepBatchSize,
		CleanupTimeout:          cfg.CleanupTimeout,
		PartialSweepGracePeriod: cfg.PartialSweepGracePeriod,
	}
}
