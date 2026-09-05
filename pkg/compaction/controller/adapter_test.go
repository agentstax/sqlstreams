package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/compaction/controller/datastore"
)

type adapterMessage struct {
	DeviceId string `json:"device_id"`
}

func (adapterMessage) SchemaVersion() int { return 1 }

func TestToStoredMessage(t *testing.T) {
	createdAt := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	data := &datastore.MessageLogRow{
		Id:             901,
		Payload:        json.RawMessage(`{"device_id":"dev-7"}`),
		CreatedAt:      createdAt,
		RoutingKey:     "devices.us-east",
		MessageKey:     "dev-7",
		CompactionRank: 3,
	}

	got, err := toStoredMessage[adapterMessage](data)
	if err != nil {
		t.Fatalf("toStoredMessage() = %v", err)
	}
	if got.Id != 901 || got.Message.DeviceId != "dev-7" || got.CreatedAt != createdAt || got.RoutingKey != "devices.us-east" || got.MessageKey != "dev-7" || got.CompactionRank != 3 {
		t.Fatalf("toStoredMessage() = %+v", got)
	}
}

func TestToStoredMessageRejectsInvalidPayload(t *testing.T) {
	_, err := toStoredMessage[adapterMessage](&datastore.MessageLogRow{Payload: json.RawMessage(`{`)})
	if err == nil {
		t.Fatal("toStoredMessage() = nil error, want invalid JSON error")
	}
}
