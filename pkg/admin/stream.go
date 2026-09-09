package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamMigrations "github.com/agentstax/sqlstreams/pkg/stream/migrations"
)

// GetStream resolves a stream by name. Returns (nil, nil), not an error,
// if name isn't registered.
func (a *MessageAdmin) GetStream(ctx context.Context, name string) (*stream.Stream, error) {
	return a.streamController.Get(ctx, name)
}

// ListStreams returns every registered stream, ordered by name.
func (a *MessageAdmin) ListStreams(ctx context.Context) ([]*stream.Stream, error) {
	return a.streamController.List(ctx)
}

// RegisterStream creates the named stream if it doesn't exist and returns
// it. Safe to call on every startup: cfg is applied on every call, so changing
// a value and redeploying changes the stream -- and two services passing
// different cfg for one stream will overwrite each other. Against an empty
// database it first stands up the control-plane tables -- RegisterSystem
// with a nil cfg.
//   - name: must match ^[a-z0-9._-]+$; dot-namespaced by domain and entity
//     ("orders.created", "billing.invoice.paid"); safe to rename later --
//     streams are addressed by id internally, not name
//   - cfg: may be nil or sparse
//
// PartitionSize is fixed at creation; passing a different one returns
// ErrStreamConfigMismatch.
// Name and config validation precede system bootstrap; later write failures
// can leave partial registration progress.
func (a *MessageAdmin) RegisterStream(ctx context.Context, name string, cfg *stream.StreamConfig) (*stream.Stream, error) {
	if name == "" {
		return nil, errors.New("stream name is required")
	}
	if isReservedStreamName(name) {
		return nil, stream.ErrReservedStreamName.With("stream", name)
	}
	if !stream.SlugPattern.MatchString(name) {
		return nil, fmt.Errorf("name must match %s, got %q", stream.SlugPattern, name)
	}
	if cfg == nil {
		cfg = &stream.StreamConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// no system row means an empty database -- the first stream stands up the
	// control-plane tables with defaults; a customized system keeps its
	// declaration, since nothing runs when the row exists
	sys, err := a.systemController.Get(ctx)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		if err := a.RegisterSystem(ctx, nil); err != nil {
			return nil, err
		}
	}

	return a.registerStream(ctx, name, cfg)
}

func (a *MessageAdmin) registerStream(ctx context.Context, name string, cfg *stream.StreamConfig) (*stream.Stream, error) {
	// gate -- a stream can't exist without the control-plane tables it rides on;
	// otherwise RegisterStream dies with a raw undefined-table error.
	sys, err := a.systemController.Get(ctx)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		return nil, migrate.ErrNotRegistered.With("stream", name)
	}

	return a.streamController.Register(ctx, sys.Id, name, cfg)
}

// MigrateStream moves the named stream's tables to targetVersion.
// Returns ErrStreamNotFound if name isn't registered.
func (a *MessageAdmin) MigrateStream(ctx context.Context, name string, targetVersion int64) error {
	owner, err := a.StreamOwner(ctx, name)
	if err != nil {
		return err
	}
	return a.migrateController.RunOnce(ctx, targetVersion, owner, streamMigrations.Registry)
}

// StreamMigrationVersion reads the version the named stream's tables are at.
// Returns ErrStreamNotFound if name isn't registered.
func (a *MessageAdmin) StreamMigrationVersion(ctx context.Context, name string) (int64, error) {
	owner, err := a.StreamOwner(ctx, name)
	if err != nil {
		return 0, err
	}
	return a.migrateController.StreamVersion(ctx, owner.StreamId)
}

// MigrateStreams moves every registered stream's schema to targetVersion.
// A no-op, not an error, if no streams are registered.
func (a *MessageAdmin) MigrateStreams(ctx context.Context, targetVersion int64) error {
	return a.migrateController.RunAll(ctx, targetVersion, common.OwnerStream, streamMigrations.Registry)
}

// RenameStream changes the stream's name. Returns ErrStreamNotFound if name
// isn't registered, ErrStreamNameTaken if newName already is.
//
// Running producers/consumers keep working (they resolved the id at their Register),
// but anything still CONFIGURED with the old name fails its next restart's Register.
func (a *MessageAdmin) RenameStream(ctx context.Context, name string, newName string) (*stream.Stream, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if newName == "" {
		return nil, errors.New("newName is required")
	}
	if isReservedStreamName(name) || isReservedStreamName(newName) {
		return nil, stream.ErrReservedStreamName.With("stream", name, "new_name", newName)
	}

	renamed, err := a.streamController.Rename(ctx, name, newName)
	if err != nil {
		return nil, err
	}
	if renamed == nil {
		return nil, stream.ErrStreamNotFound.With("stream", name)
	}
	return renamed, nil
}

// DestroyOptions configures one Destroy call on a stream, consumer, or the
// system. Every destroy is refused unless ClientConfig.AllowDestroy is set.
type DestroyOptions struct {
	// Force - skips the in-use guard: a stream still holding messages, a
	// consumer group with a live instance or delivery rows, a system with a
	// live worker instance or a stream registered.
	// Default: false.
	Force bool
}

// DestroyStream permanently drops the named stream and every message it
// holds. Returns stream.ErrDestroyDisabled unless
// MessageAdminConfig.AllowDestroy is set, ErrStreamNotFound if name isn't
// registered, and ErrStreamNotEmpty if the stream still holds
// messages and options.Force isn't set.
func (a *MessageAdmin) DestroyStream(ctx context.Context, name string, options *DestroyOptions) error {
	if !a.allowDestroy {
		return stream.ErrDestroyDisabled
	}
	if options == nil {
		options = &DestroyOptions{}
	}
	if name == "" {
		return errors.New("stream name is required")
	}
	if isReservedStreamName(name) {
		return stream.ErrReservedStreamName.With("stream", name)
	}

	found, err := a.streamController.Get(ctx, name)
	if err != nil {
		return err
	}
	if found == nil {
		return stream.ErrStreamNotFound.With("stream", name)
	}

	if !options.Force {
		if err := a.assertStreamIdle(ctx, found.Id, found.Name); err != nil {
			return err
		}
	}

	return a.streamController.Delete(ctx, found.Id, found.Name)
}

// assertStreamIdle is DestroyStream's guard: no message would be discarded.
func (a *MessageAdmin) assertStreamIdle(ctx context.Context, streamId int64, name string) error {
	empty, err := a.streamController.IsEmpty(ctx, streamId)
	if err != nil {
		return err
	}
	if !empty {
		return stream.ErrStreamNotEmpty.With("stream", name, "stream_id", streamId)
	}
	return nil
}

// ***************
// *** HELPERS ***
// ***************

func isReservedStreamName(name string) bool {
	return strings.HasPrefix(name, common.SystemStreamPrefix)
}
