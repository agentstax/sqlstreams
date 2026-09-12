package produce

import (
	"context"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

// ProducerFunc runs inside the append's transaction and returns the payload to
// store -- its writes commit or roll back with the message.
type ProducerFunc[Message common.Versioned] func(ctx context.Context, tx datastore.Tx) (*Message, error)
