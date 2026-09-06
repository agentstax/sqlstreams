package datastore

import (
	"uuid"

	"github.com/agentstax/vulkan/pkg/common"
)

// Append is one message append's inputs, insert-arg exact.
type Append[Message common.Versioned] struct {
	// resolved once by the caller and reused across every retry -- that reuse
	// is what makes a retried attempt safe after an ambiguous commit
	IdempotencyKey uuid.UUID
	Payload        *Message // nil when a ProduceFunc supplies it inside the transaction
	RoutingKey     string
	MessageKey     string
	Compacted      bool  // the produce enabled Compaction
	CompactionRank int64 // read only when Compacted
	Options        *common.MessageOptions
}

// Appended is one append's outcome.
type Appended[Message common.Versioned] struct {
	Message   *Message // the payload this call built -- never re-read from storage
	Id        int64    // 0 when Duplicate
	Duplicate bool     // the idempotency claim already existed
}
