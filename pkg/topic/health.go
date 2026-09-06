package topic

import (
	"github.com/agentstax/vulkan/pkg/metrics"
)

// TopicVersionHealth is one payload version's retire verdict on a topic: safe
// once no compaction head points at it and every group has read past it.
type TopicVersionHealth struct {
	Topic           *Topic                                  `json:"topic"`
	Version         int                                     `json:"version"`
	Messages        int64                                   `json:"messages"`
	CompactionHeads int64                                   `json:"compaction_heads"`
	Groups          []metrics.ConsumerGroupSchemaVersionLag `json:"groups"`
	Safe            bool                                    `json:"safe"`
	Reason          string                                  `json:"reason"`
}
