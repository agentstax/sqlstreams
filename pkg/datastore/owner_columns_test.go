package datastore

import (
	"testing"

	"github.com/agentstax/vulkan/pkg/common"
)

func TestOwnerColumnsSelectsOnlyOwningResource(t *testing.T) {
	for _, sample := range []struct {
		name  string
		owner common.Owner
		want  [3]int64
	}{
		{"system", common.Owner{SystemId: 1}, [3]int64{1, 0, 0}},
		{"topic", common.Owner{SystemId: 1, TopicId: 12}, [3]int64{0, 12, 0}},
		{"consumer group", common.Owner{SystemId: 1, TopicId: 12, ConsumerGroupId: 34}, [3]int64{0, 0, 34}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			columns := NewOwnerColumns(sample.owner)
			for index, value := range []*int64{columns.SystemId, columns.TopicId, columns.ConsumerGroupId} {
				if sample.want[index] == 0 {
					if value != nil {
						t.Fatalf("column %d should be SQL NULL, got %d", index, *value)
					}
				} else if value == nil || *value != sample.want[index] {
					t.Fatalf("column %d should hold owner ID %d", index, sample.want[index])
				}
			}
		})
	}
}
