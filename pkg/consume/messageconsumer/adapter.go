package messageconsumer

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/messageconsumer/controller"
)

func toMessageConsumerMetadata(cfg *MessageConsumerConfig) *MessageConsumerMetadata {
	return &MessageConsumerMetadata{
		Message:                 cfg.Message,
		MessageMin:              cfg.MessageMin,
		MessageMax:              cfg.MessageMax,
		ConcurrencyOverride:     cfg.ConcurrencyOverride,
		ExceptionInitialBackoff: cfg.ExceptionInitialBackoff,
		MaxRangeReclaims:        cfg.MaxRangeReclaims,
	}
}

func toMessageMeta(claimed controller.Message, resolved *common.MessageOptions) consume.MessageMeta {
	return consume.MessageMeta{
		Id:             claimed.Id,
		RoutingKey:     claimed.RoutingKey,
		MessageKey:     claimed.MessageKey,
		CompactionRank: claimed.CompactionRank,
		CreatedAt:      claimed.CreatedAt,
		ScheduledAt:    resolved.ScheduledAt,
		Options:        resolved,
	}
}
