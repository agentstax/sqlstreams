package controller

import (
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	"github.com/jackc/pgx/v5/pgtype"
)

func toKeyLeaseClaim(data *datastore.KeyLease) *KeyLeaseClaim {
	return &KeyLeaseClaim{
		Verdict:         KeyLeaseVerdict(data.Verdict),
		StreamId:        data.StreamId,
		ConsumerGroupId: data.ConsumerGroupId,
		MessageKey:      data.MessageKey,
		Token:           uuid.UUID(data.Token.Bytes),
	}
}

func toKeyLease(claim *KeyLeaseClaim) *datastore.KeyLease {
	return &datastore.KeyLease{
		Verdict:         datastore.KeyLeaseVerdict(claim.Verdict),
		StreamId:        claim.StreamId,
		ConsumerGroupId: claim.ConsumerGroupId,
		MessageKey:      claim.MessageKey,
		Token:           toTokenData(claim.Token),
	}
}

func toTokenData(token uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: token, Valid: true}
}
