package system

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// ErrSystemLive means DestroySystem was refused because a worker instance is
// still live -- a manager or consumer is running somewhere.
var ErrSystemLive = diagnostic.NewDiagnosticError("SS0010", diagnostic.RecoveryPermanent,
	"a worker instance is still live",
	"stop running managers and consumers, or pass DestroyOptions.Force")

// ErrStreamsRegistered means DestroySystem was refused because non-system
// streams are still registered.
var ErrStreamsRegistered = diagnostic.NewDiagnosticError("SS0011", diagnostic.RecoveryPermanent,
	"streams are still registered",
	"destroy them first, or pass DestroyOptions.Force to destroy them and their messages")

// ErrSchemaNotCreatable means RegisterSystem could not create the namespace
// sqlstreams's tables live in -- the connecting role has no CREATE privilege.
var ErrSchemaNotCreatable = diagnostic.NewDiagnosticError("SS0064", diagnostic.RecoveryPermanent,
	"the connecting role cannot create the schema",
	"grant it CREATE on the database, or create schema {schema} yourself and grant the role USAGE and CREATE on it")
