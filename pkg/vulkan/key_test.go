package vulkan

import (
	"context"
	"testing"
)

func TestKeyHandleNamesResourceWithoutIO(t *testing.T) {
	client := &Client{}
	key := client.Topic[RawPayload]("devices.config").Key("dev-7")

	if key.topicName != "devices.config" {
		t.Fatalf("topicName = %q, want %q", key.topicName, "devices.config")
	}
	if key.messageKey != "dev-7" {
		t.Fatalf("messageKey = %q, want %q", key.messageKey, "dev-7")
	}
}

func TestKeyHandleLockCompactionHeadFacadeSignature(t *testing.T) {
	var lock func(*KeyHandle[RawPayload], context.Context, Tx) (*StoredMessage[RawPayload], error)
	lock = (*KeyHandle[RawPayload]).LockCompactionHead
	if lock == nil {
		t.Fatal("LockCompactionHead method expression is nil")
	}
}
