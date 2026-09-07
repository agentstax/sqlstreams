package controller

import (
	"testing"

	"github.com/agentstax/vulkan/pkg/common"
)

func TestEvaluateValidatesBeforeReading(t *testing.T) {
	owner, err := common.NewTopicOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	controller := &PartitionCountController{}
	if _, err := controller.Evaluate(t.Context(), nil, 0); err == nil {
		t.Fatal("nil owner must fail before database access")
	}
	if _, err := controller.Evaluate(t.Context(), owner, -1); err == nil {
		t.Fatal("negative threshold must fail before database access")
	}
}
