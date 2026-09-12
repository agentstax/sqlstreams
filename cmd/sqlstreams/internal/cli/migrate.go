package cli

import (
	"context"
	"errors"

	sqlstreams "github.com/agentstax/sqlstreams/client"
	streamMigrations "github.com/agentstax/sqlstreams/pkg/stream/migrations"
	systemMigrations "github.com/agentstax/sqlstreams/pkg/system/migrations"
	"github.com/spf13/cobra"
)

// availableSystemVersion / availableStreamVersion are the version ceilings this
// binary knows: the v1 baseline plus every step compiled into the registry. The
// registry is the source of truth, not the DB -- an older CLI against a newer DB
// reports its own lower ceiling, which is information, not an error.
func availableSystemVersion() int64 { return int64(len(systemMigrations.Registry)) + 1 }
func availableStreamVersion() int64 { return int64(len(streamMigrations.Registry)) + 1 }

func newMigrateCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Initialize and version the control-plane and stream schemas",
	}

	cmd.AddCommand(newMigrateInitCmd(g))
	cmd.AddCommand(newMigrateVersionsCmd(g))
	cmd.AddCommand(newMigrateStatusCmd(g))
	cmd.AddCommand(newMigrateSystemCmd(g))
	cmd.AddCommand(newMigrateStreamsCmd(g))
	cmd.AddCommand(newMigrateStreamCmd(g))

	return cmd
}

// scope is which schema a migrate command targets.
type scope int

const (
	scopeSystem  scope = iota // the shared control-plane tables (a singleton)
	scopeStreams              // every registered stream
	scopeStream               // one stream, by name
)

// direction is the guardrail the operator committed to on the command line.
// It's not passed to the controller (which infers up/down from target vs current);
// it's enforced CLI-side so `down` can never silently roll a migration forward.
type direction int

const (
	dirUp direction = iota
	dirDown
)

func (d direction) verb() string {
	if d == dirDown {
		return "down"
	}
	return "up"
}

// ceiling is the highest version this binary can migrate a scope to.
func (s scope) ceiling() int64 {
	if s == scopeSystem {
		return availableSystemVersion()
	}
	return availableStreamVersion()
}

// migrateTarget is one target a run touches, paired with its current DB version
// so the direction guard and the no-op check can reason about it before any DDL.
type migrateTarget struct {
	name    string
	current int64
}

// gatherTargets resolves the targets a scope covers and reads each one's current
// schema version. Registration gaps surface here as teaching errors, before the
// migrate call, so the operator never sees a raw undefined-table or ErrNotRegistered.
func gatherTargets(ctx context.Context, client *sqlstreams.Client, s scope, name string) ([]migrateTarget, error) {
	switch s {
	case scopeSystem:
		current, err := client.System().MigrationVersion(ctx)
		if err != nil {
			if errors.Is(err, sqlstreams.ErrNotRegistered) {
				return nil, errSystemNotRegistered()
			}
			return nil, translateAdminError(err)
		}
		return []migrateTarget{{name: "system", current: current}}, nil

	case scopeStream:
		current, err := client.Stream[sqlstreams.RawPayload](name).MigrationVersion(ctx)
		if err != nil {
			if errors.Is(err, sqlstreams.ErrStreamNotFound) {
				return nil, failOp("stream %q not found", name)
			}
			return nil, translateAdminError(err)
		}
		return []migrateTarget{{name: name, current: current}}, nil

	default: // scopeStreams
		streams, err := client.Streams(ctx)
		if err != nil {
			return nil, translateAdminError(err)
		}
		targets := make([]migrateTarget, 0, len(streams))
		for _, t := range streams {
			current, err := client.Stream[sqlstreams.RawPayload](t.Name).MigrationVersion(ctx)
			if err != nil {
				return nil, translateAdminError(err)
			}
			targets = append(targets, migrateTarget{name: t.Name, current: current})
		}
		return targets, nil
	}
}

// guardDirection rejects a target that sits on the wrong side of the operator's
// chosen direction: `up` must never roll a migration back, `down` must always. The
// controller would happily do either from a bare target -- this is what makes the
// explicit up/down split mean something. Returns the count of targets that will
// actually move (target != current) so the caller can no-op cleanly.
func guardDirection(targets []migrateTarget, dir direction, to int64) (moving int, err error) {
	for _, t := range targets {
		switch {
		case dir == dirUp && to < t.current:
			return 0, failUsage("%s is at version %d; --to %d is a downgrade -- use `down` to roll back", t.name, t.current, to)
		case dir == dirDown && to > t.current:
			return 0, failUsage("%s is at version %d; --to %d is not a downgrade -- use `up` to move forward", t.name, t.current, to)
		}
		if to != t.current {
			moving++
		}
	}
	return moving, nil
}

// errSystemNotRegistered is the single teaching error every path raises when the
// control-plane tables is missing -- one wording, one place to change it.
func errSystemNotRegistered() error {
	return failUsage("system not registered -- run `sqlstreams migrate init` first")
}

// migrateError maps a failed admin migrate call to operator-facing output. Most
// preconditions are caught before the call (gatherTargets/guardDirection); this
// covers the residue -- a lost registration race, or a step that errored midway.
func migrateError(err error) error {
	if errors.Is(err, sqlstreams.ErrNotRegistered) {
		return errSystemNotRegistered()
	}
	return translateAdminError(err)
}
