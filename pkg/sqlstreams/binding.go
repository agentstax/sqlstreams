package sqlstreams

import (
	"context"
)

// BindingHandle is a consumer group's binding declaration,
// named on its stream and group, holding no row. Get is the comma-ok read.
type BindingHandle struct {
	streamName string
	groupName  string
	client     *Client
}

// Bindings returns every group's effective binding declaration
// and any declarers still waiting to change it, ordered by stream then
// group. A group that never declared a set reads the whole stream and does
// not appear.
func (s *SystemHandle) Bindings(ctx context.Context) ([]*Binding, error) {
	return s.client.admin.ListBindings(ctx)
}

// Binding names this group's binding declaration. No I/O and
// no failure -- Get resolves both names when called.
func (h *ConsumerHandle[Message]) Binding() *BindingHandle {
	return &BindingHandle{streamName: h.streamName, groupName: h.name, client: h.client}
}

// Get reads the group's effective declaration -- its newest installed
// set. Returns (nil, nil) when the stream or the group is not registered,
// or when the group never declared a set and reads the whole stream.
func (b *BindingHandle) Get(ctx context.Context) (*Binding, error) {
	return b.client.admin.GetBinding(ctx, b.streamName, b.groupName)
}
