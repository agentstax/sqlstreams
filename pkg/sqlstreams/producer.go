package sqlstreams

import "context"

// ProducerHandle is a stream's name plus the client, holding no row.
type ProducerHandle[Message Versioned] struct {
	streamName string
	client     *Client
}

// Producer names this stream as a produce target. No I/O and no failure --
// Register resolves the stream when called.
func (t *StreamHandle[Message]) Producer() *ProducerHandle[Message] {
	return &ProducerHandle[Message]{streamName: t.name, client: t.client}
}

// Register resolves the stream and returns an instance that produces its
// Message. cfg may be nil or sparse.
func (p *ProducerHandle[Message]) Register(ctx context.Context, cfg *ProducerConfig) (*ProducerInstance[Message], error) {
	instance, err := p.client.producer.Register[Message](ctx, p.streamName, cfg)
	if err != nil {
		return nil, err
	}
	return newProducerInstance(instance)
}
