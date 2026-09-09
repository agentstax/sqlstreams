package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/metric"
)

// MetricHandle is one exact measurement series, holding no database row.
type MetricHandle struct {
	declared   *diagnostic.DiagnosticMetric
	messageKey string
	client     *Client
}

func newMetricHandle(client *Client, declared *diagnostic.DiagnosticMetric, name string, attributes map[string]string) *MetricHandle {
	return &MetricHandle{
		declared:   declared,
		messageKey: metric.MeasurementKey(name, attributes),
		client:     client,
	}
}

// Latest returns the series' newest retained measurement, or nil if no
// retained measurement has its key.
func (m *MetricHandle) Latest(ctx context.Context) (*Measurement, error) {
	stored, err := m.client.admin.GetMeasurement(ctx, m.messageKey)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, nil
	}
	return stored.Message, nil
}

// History returns the series' retained measurements newest first. limit must
// be positive.
func (m *MetricHandle) History(ctx context.Context, limit int) ([]*Measurement, error) {
	stored, err := m.client.admin.ListMeasurementMessages(ctx, m.messageKey, limit)
	if err != nil {
		return nil, err
	}
	return unwrapMessages(stored), nil
}

// ***************
// *** HELPERS ***
// ***************

func unwrapMessages[Message Versioned](stored []*StoredMessage[Message]) []*Message {
	messages := make([]*Message, 0, len(stored))
	for _, message := range stored {
		messages = append(messages, message.Message)
	}
	return messages
}
