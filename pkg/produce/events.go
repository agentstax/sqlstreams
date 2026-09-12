package produce

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
)

// EventPartitionNotCreatedAhead means the create-ahead pass gave up on the
// next partition; the write path still covers it.
var EventPartitionNotCreatedAhead = diagnostic.NewDiagnosticEvent("SQL0033",
	"could not create partition ahead",
	"the first insert past the boundary will create it")

// EventPartitionCreatedOnInsert means an insert found no partition for its
// id and created one itself: create-ahead did not run, or a burst
// outran its triggers.
var EventPartitionCreatedOnInsert = diagnostic.NewDiagnosticEvent("SQL0057",
	"no partition covers the next message id",
	"the insert creates it and pays the creation latency; run a consumer for the stream's upkeep or raise PartitionSize")

// EventSlowProduce means one produce call ran past the producer's
// SlowProduceThreshold, whatever the call's outcome.
var EventSlowProduce = diagnostic.NewDiagnosticEvent("SQL0038",
	"produce exceeded the duration threshold", "")
