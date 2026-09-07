package collectorprogress

func toCollectorProgressMetadata(cfg *CollectorProgressConfig) *collectorProgressMetadata {
	return &collectorProgressMetadata{
		RepeatInterval: cfg.RepeatInterval,
	}
}
