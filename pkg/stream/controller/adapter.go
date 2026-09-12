package controller

import (
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream/controller/datastore"
)

func toStream(data *datastore.StreamConfigRow) (*stream.Stream, error) {
	deliveryLogMode, err := deliveryLogModeEnum(data.DeliveryLogMode)
	if err != nil {
		return nil, err
	}

	return &stream.Stream{
		Id:                     data.Id,
		SystemId:               data.SystemId,
		Name:                   data.Name,
		PartitionSize:          data.PartitionSize,
		RetentionTTL:           time.Duration(data.RetentionTTLNs),
		AllowDropPastCommitted: data.AllowDropPastCommitted,
		IdempotencyKeyTTL:      time.Duration(data.IdempotencyKeyTTLNs),
		EmptyCompactionHeadTTL: time.Duration(data.EmptyCompactionHeadTTLNs),
		DeliveryLogMode:        deliveryLogMode,
	}, nil
}

func toStreamConfigRow(systemId int64, name string, cfg *stream.StreamConfig) *datastore.StreamConfigRow {
	return &datastore.StreamConfigRow{
		SystemId:                 systemId,
		Name:                     name,
		PartitionSize:            cfg.PartitionSize,
		RetentionTTLNs:           int64(cfg.RetentionTTL),
		AllowDropPastCommitted:   cfg.AllowDropPastCommitted,
		IdempotencyKeyTTLNs:      int64(cfg.IdempotencyKeyTTL),
		EmptyCompactionHeadTTLNs: int64(cfg.EmptyCompactionHeadTTL),
		DeliveryLogMode:          string(cfg.DeliveryLogMode),
	}
}

func deliveryLogModeEnum(deliveryLogMode string) (stream.DeliveryLogMode, error) {
	switch stream.DeliveryLogMode(deliveryLogMode) {
	case stream.DeliveryLogModeOff, stream.DeliveryLogModeFailures, stream.DeliveryLogModeAll:
		return stream.DeliveryLogMode(deliveryLogMode), nil
	default:
		return "", fmt.Errorf("stored delivery_log_mode %q is not %q, %q, or %q", deliveryLogMode, stream.DeliveryLogModeOff, stream.DeliveryLogModeFailures, stream.DeliveryLogModeAll)
	}
}
