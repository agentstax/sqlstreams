package controller

import (
	"context"
	"fmt"

	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

func (c *TopicController) AssertSchemaSupported(ctx context.Context, systemId int64, topicId int64) error {
	if systemId <= 0 {
		return fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if topicId <= 0 {
		return fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	return c.migrateController.AssertTopicSchemaSupported(ctx, systemId, topicId)
}

// AssertSchemaSupportedInTx gates a topic through tx.
func (c *TopicController) AssertSchemaSupportedInTx(ctx context.Context, tx iDatastore.Tx, systemId int64, topicId int64) error {
	return c.migrateController.AssertTopicSchemaSupportedInTx(ctx, tx, systemId, topicId)
}
