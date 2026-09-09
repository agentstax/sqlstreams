package metric

import (
	"time"

	"github.com/agentstax/vulkan/pkg/common"
)

// MeasurementHistory contains retained messages in a database-time window,
// ordered by CreatedAt then id descending, including superseded rows.
type MeasurementHistory struct {
	EvaluatedAt time.Time                            `json:"evaluated_at"` // retrieved from db for clock bs reasons
	Messages    []*common.StoredMessage[Measurement] `json:"messages"`
}
