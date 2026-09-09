package common

import (
	"errors"
	"fmt"
	"hash/crc32"
)

// AdvisoryLockNamespace is the high 32 bits of every advisory lock key sqlstreams
// takes -- ASCII "VULK", 1448430667. 4 bytes specifically.
const AdvisoryLockNamespace int64 = 0x56554C4B

// AdvisoryLockKey is one advisory lock's key
type AdvisoryLockKey struct {
	value int64
}

// NewAdvisoryLockKey derives the key for one advisory lock: the namespace in
// the high 32 bits, a crc32 checksum of "kind:schema[:part...]" in the low 32.
// Different schemas checksum differently, so two installations in one database
// never wait on each other's locks.
//
// How the bytes move, for ("stream", "sqlstreams", "orders") -- the checksum of
// "stream:sqlstreams:orders" is 0xAB15D0C1 (2870333633):
//
//	namespace        00 00 00 00 56 55 4C 4B
//	namespace << 32  56 55 4C 4B 00 00 00 00     ← slid left, low half zeroed
//	int64(checksum)  00 00 00 00 AB 15 D0 C1     ← uint32 fills exactly 4 bytes
//	OR of the two    56 55 4C 4B AB 15 D0 C1     = 6220962348158800065, the key
//	                 └─classid─┘ └──objid──┘
//	                  1448430667  2870333633     how pg_locks files the halves
func NewAdvisoryLockKey(kind string, schema string, parts ...any) (*AdvisoryLockKey, error) {
	if kind == "" {
		return nil, errors.New("kind must not be empty")
	}
	if schema == "" {
		return nil, errors.New("schema must not be empty")
	}

	name := kind + ":" + schema
	for _, part := range parts {
		name += fmt.Sprintf(":%v", part)
	}

	return &AdvisoryLockKey{value: AdvisoryLockNamespace<<32 | int64(crc32.ChecksumIEEE([]byte(name)))}, nil
}

// Value is the whole key, what pg_advisory_lock and its siblings take.
func (k AdvisoryLockKey) Value() int64 {
	return k.value
}

// ClassId is the high half, what pg_locks.classid reports the key under.
func (k AdvisoryLockKey) ClassId() int64 {
	return k.value >> 32
}

// ObjId is the low half, what pg_locks.objid reports the key under.
func (k AdvisoryLockKey) ObjId() int64 {
	return k.value & 0xFFFFFFFF
}
