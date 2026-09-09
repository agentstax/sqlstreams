package collector

func toMetricsCollectorMetadata(cfg *MetricCollectorConfig) *metricCollectorMetadata {
	return &metricCollectorMetadata{PollRate: cfg.PollRate}
}
