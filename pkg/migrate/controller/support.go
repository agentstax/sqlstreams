package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	"github.com/agentstax/sqlstreams/pkg/migrate/controller/datastore"
	streamMigrations "github.com/agentstax/sqlstreams/pkg/stream/migrations"
	systemMigrations "github.com/agentstax/sqlstreams/pkg/system/migrations"
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

// AssertStreamSchemaSupported gates startup for a stream- or group-owned
// caller against both the shared system tables and the stream's own tables.
func (c *Controller) AssertStreamSchemaSupported(ctx context.Context, systemId int64, streamId int64) error {
	if err := c.AssertSystemSchemaSupported(ctx, systemId); err != nil {
		return err
	}
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	state, err := c.datastore.StreamSchemaState(ctx, streamId)
	if err != nil {
		return err // ErrNotRegistered, or a real db error
	}
	return assertVersionSupported(common.OwnerStream, state, streamMigrations.Version())
}

// AssertStreamSchemaSupportedInTx gates a stream-owned caller through tx, so
// schema compatibility and the caller's following work share one transaction.
func (c *Controller) AssertStreamSchemaSupportedInTx(ctx context.Context, tx iDatastore.Tx, systemId int64, streamId int64) error {
	if tx == nil {
		return errors.New("tx must not be nil")
	}
	if systemId <= 0 {
		return fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	state, err := c.datastore.SystemSchemaStateInTx(ctx, tx, systemId)
	if err != nil {
		return err
	}
	if err := assertVersionSupported(common.OwnerSystem, state, systemMigrations.Version()); err != nil {
		return err
	}

	state, err = c.datastore.StreamSchemaStateInTx(ctx, tx, streamId)
	if err != nil {
		return err
	}
	return assertVersionSupported(common.OwnerStream, state, streamMigrations.Version())
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
