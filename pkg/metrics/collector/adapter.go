package collector

func toMetricsCollectorMetadata(cfg *MetricsCollectorConfig) *metricsCollectorMetadata {
	return &metricsCollectorMetadata{PollRate: cfg.PollRate}
}
