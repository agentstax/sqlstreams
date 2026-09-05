package controller

import (
	"strings"
	"testing"

	"github.com/agentstax/vulkan/pkg/common"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

type validationTx struct {
	iDatastore.Tx
}

func TestLockHeadValidatesInputsBeforeQuerying(t *testing.T) {
	controller := &CompactionController{}
	tx := &validationTx{}
	tests := []struct {
		name       string
		tx         iDatastore.Tx
		topicId    int64
		messageKey string
		want       string
	}{
		{name: "nil transaction", topicId: 41, messageKey: "dev-7", want: "tx must not be nil"},
		{name: "invalid topic", tx: tx, messageKey: "dev-7", want: "topicId must be > 0"},
		{name: "empty key", tx: tx, topicId: 41, want: "messageKey must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := controller.LockHead[common.RawPayload](t.Context(), test.tx, test.topicId, test.messageKey)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LockHead() = %v, want error containing %q", err, test.want)
			}
		})
	}
}
