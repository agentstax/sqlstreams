package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/migrate"
	"github.com/agentstax/vulkan/pkg/migrate/controller/datastore"
	systemMigrations "github.com/agentstax/vulkan/pkg/system/migrations"
	topicMigrations "github.com/agentstax/vulkan/pkg/topic/migrations"
)

// AssertSystemSchemaSupported gates startup for a system-owned caller against
// the shared system tables.
// Too new -> upgrade the binary; too old -> migrate the database.
func (c *Controller) AssertSystemSchemaSupported(ctx context.Context, systemId int64) error {
	if systemId <= 0 {
		return fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	state, err := c.datastore.SystemSchemaState(ctx, systemId)
	if err != nil {
		return err // ErrNotRegistered, or a real db error
	}
	return assertVersionSupported(common.OwnerSystem, state, systemMigrations.Version())
}

// AssertTopicSchemaSupported gates startup for a topic- or group-owned
// caller against both the shared system tables and the topic's own tables.
func (c *Controller) AssertTopicSchemaSupported(ctx context.Context, systemId int64, topicId int64) error {
	if err := c.AssertSystemSchemaSupported(ctx, systemId); err != nil {
		return err
	}
	if topicId <= 0 {
		return fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	state, err := c.datastore.TopicSchemaState(ctx, topicId)
	if err != nil {
		return err // ErrNotRegistered, or a real db error
	}
	return assertVersionSupported(common.OwnerTopic, state, topicMigrations.Version())
}

// AssertTopicSchemaSupportedInTx gates a topic-owned caller through tx, so
// schema compatibility and the caller's following work share one transaction.
func (c *Controller) AssertTopicSchemaSupportedInTx(ctx context.Context, tx iDatastore.Tx, systemId int64, topicId int64) error {
	if tx == nil {
		return errors.New("tx must not be nil")
	}
	if systemId <= 0 {
		return fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if topicId <= 0 {
		return fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	state, err := c.datastore.SystemSchemaStateInTx(ctx, tx, systemId)
	if err != nil {
		return err
	}
	if err := assertVersionSupported(common.OwnerSystem, state, systemMigrations.Version()); err != nil {
		return err
	}

	state, err = c.datastore.TopicSchemaStateInTx(ctx, tx, topicId)
	if err != nil {
		return err
	}
	return assertVersionSupported(common.OwnerTopic, state, topicMigrations.Version())
}

// ***************
// *** HELPERS ***
// ***************

// assertVersionSupported renders migrate.ClassifySchemaSupport's answer as
// the declared error for whichever side is behind. buildVersion is what this
// binary's registry defines; state is what the database records.
func assertVersionSupported(kind common.OwnerKind, state *datastore.SchemaStateRow, buildVersion int64) error {
	switch migrate.ClassifySchemaSupport(state.Version, state.MinCompatibleVersion, buildVersion) {
	case migrate.SchemaOlderThanBuild:
		return migrate.ErrSchemaOlderThanBuild.With("owner_kind", kind, "version", state.Version, "build_version", buildVersion)
	case migrate.SchemaNewerThanBuild:
		return migrate.ErrSchemaNewerThanBuild.With("owner_kind", kind, "version", state.Version, "min_compatible_version", state.MinCompatibleVersion, "build_version", buildVersion)
	}
	return nil
}
