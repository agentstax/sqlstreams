package admin

import (
	"context"
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// SystemOwner resolves the system to its owner. Returns ErrNotRegistered
// until RegisterSystem has run.
func (a *MessageAdmin) SystemOwner(ctx context.Context) (*common.Owner, error) {
	sys, err := a.GetSystem(ctx)
	if err != nil {
		return nil, err
	}
	return common.NewSystemOwner(sys.Id)
}

// StreamOwner resolves the stream registered under name to its owner. Returns
// ErrStreamNotFound when it is missing.
func (a *MessageAdmin) StreamOwner(ctx context.Context, name string) (*common.Owner, error) {
	found, err := a.GetStream(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", name)
	}
	return common.NewStreamOwner(found.SystemId, found.Id, found.Name)
}

// ConsumerGroupOwner resolves the group registered under consumerName on streamName to
// its owner. Returns ErrStreamNotFound / ErrConsumerNotFound when either side
// is missing.
func (a *MessageAdmin) ConsumerGroupOwner(ctx context.Context, streamName string, consumerName string) (*common.Owner, error) {
	if consumerName == "" {
		return nil, errors.New("consumer name is required")
	}

	found, err := a.GetStream(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}

	consumerGroup, err := a.consumerController.GetGroup(ctx, found.Id, consumerName)
	if err != nil {
		return nil, err
	}
	if consumerGroup == nil {
		return nil, consume.ErrConsumerNotFound.With("group", consumerName, "stream", streamName)
	}
	return common.NewConsumerGroupOwner(found.SystemId, found.Id, consumerGroup.Id, consumerGroup.Name)
}
