package controller

import (
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

func StreamConfig() *stream.StreamConfig {
	return &stream.StreamConfig{
		PartitionSize:   10_000,
		RetentionTTL:    7 * 24 * time.Hour,
		DeliveryLogMode: stream.DeliveryLogModeOff,
	}
}
