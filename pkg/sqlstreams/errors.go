package sqlstreams

// Every declared error, under its own name: the same value, so errors.Is
// holds whether a caller spells the root or this package.

import (
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/compaction"
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/system"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

var (
	ErrAlreadyConsuming               = common.ErrAlreadyConsuming
	ErrCommitConfirmationLost         = common.ErrCommitConfirmationLost
	ErrLeaseLost                      = common.ErrLeaseLost
	ErrLifecycleContextNotCancellable = common.ErrLifecycleContextNotCancellable
	ErrPayloadNotEncodable            = common.ErrPayloadNotEncodable
	ErrCompactionHeadNotFound         = compaction.ErrCompactionHeadNotFound
	ErrDeliveryDelayed                = consume.ErrDeliveryDelayed
	ErrDeliveryTerminal               = consume.ErrDeliveryTerminal
	ErrConsumerGroupDeliveriesPending = consume.ErrConsumerGroupDeliveriesPending
	ErrConsumerGroupLive              = consume.ErrConsumerGroupLive
	ErrConsumerNotFound               = consume.ErrConsumerNotFound
	ErrNotRegistered                  = migrate.ErrNotRegistered
	ErrSchemaNewerThanBuild           = migrate.ErrSchemaNewerThanBuild
	ErrSchemaOlderThanBuild           = migrate.ErrSchemaOlderThanBuild
	ErrStepLockTimeout                = migrate.ErrStepLockTimeout
	ErrPartitionCreationBehind        = produce.ErrPartitionCreationBehind
	ErrPartitionLockTimeout           = produce.ErrPartitionLockTimeout
	ErrScheduleDeclarationInterrupted = schedule.ErrScheduleDeclarationInterrupted
	ErrScheduleNotFound               = schedule.ErrScheduleNotFound
	ErrSchemaNotCreatable             = system.ErrSchemaNotCreatable
	ErrSystemLive                     = system.ErrSystemLive
	ErrStreamsRegistered              = system.ErrStreamsRegistered
	ErrDestroyDisabled                = stream.ErrDestroyDisabled
	ErrReservedStreamName             = stream.ErrReservedStreamName
	ErrStreamConfigMismatch           = stream.ErrStreamConfigMismatch
	ErrStreamDeclarationInterrupted   = stream.ErrStreamDeclarationInterrupted
	ErrStreamNameTaken                = stream.ErrStreamNameTaken
	ErrStreamNotEmpty                 = stream.ErrStreamNotEmpty
	ErrStreamNotFound                 = stream.ErrStreamNotFound
	ErrStreamPartitionsRemain         = stream.ErrStreamPartitionsRemain
	ErrInstanceLost                   = worker.ErrInstanceLost
	ErrWorkerDeclarationInterrupted   = worker.ErrWorkerDeclarationInterrupted
)
