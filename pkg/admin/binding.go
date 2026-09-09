package admin

import (
	"context"
	"errors"

	"github.com/agentstax/sqlstreams/pkg/consume"
)

// GetBinding reads the group's effective binding declaration --
// its newest installed set. Returns (nil, nil) when the stream or the group
// is absent, or when the group never declared a set.
func (a *MessageAdmin) GetBinding(ctx context.Context, streamName string, consumerName string) (*consume.Binding, error) {
	if consumerName == "" {
		return nil, errors.New("consumer name is required")
	}

	found, err := a.GetStream(ctx, streamName)
	if err != nil || found == nil {
		return nil, err
	}
	consumerGroup, err := a.consumerController.GetGroup(ctx, found.Id, consumerName)
	if err != nil || consumerGroup == nil {
		return nil, err
	}
	return a.consumerController.GetBinding(ctx, found.Id, consumerGroup.Id)
}

// ListBindings returns every group's effective binding declaration and
// any declarers still waiting to change it.
// Groups with empty (no) binding declaration do not show here - even though
// they work, they just match on every message in their stream.
func (a *MessageAdmin) ListBindings(ctx context.Context) ([]*consume.Binding, error) {
	return a.consumerController.ListBindings(ctx)
}
