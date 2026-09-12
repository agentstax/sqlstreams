package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// Get resolves a stream by name. Returns (nil, nil) if name is not found.
func (c *StreamController) Get(ctx context.Context, name string) (*stream.Stream, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}

	found, err := c.datastore.Get(ctx, name)
	if err != nil || found == nil {
		return nil, err
	}
	return toStream(found)
}

// GetInTx resolves a stream by name through tx. Returns (nil, nil) if name is
// not found.
func (c *StreamController) GetInTx(ctx context.Context, tx iDatastore.Tx, name string) (*stream.Stream, error) {
	if tx == nil {
		return nil, errors.New("tx must not be nil")
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	found, err := c.datastore.GetInTx(ctx, tx, name)
	if err != nil || found == nil {
		return nil, err
	}
	return toStream(found)
}

// GetById resolves a stream by its id. Returns (nil, nil) if no stream has it.
func (c *StreamController) GetById(ctx context.Context, id int64) (*stream.Stream, error) {
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0, got %d", id)
	}

	found, err := c.datastore.GetById(ctx, id)
	if err != nil || found == nil {
		return nil, err
	}
	return toStream(found)
}

func (c *StreamController) List(ctx context.Context) ([]*stream.Stream, error) {
	listed, err := c.datastore.List(ctx)
	if err != nil {
		return nil, err
	}

	var streams []*stream.Stream
	for _, data := range listed {
		listedStream, err := toStream(&data)
		if err != nil {
			return nil, err
		}
		streams = append(streams, listedStream)
	}
	return streams, nil
}

// Register resolves name to its db identity, creating the
// stream if it doesn't exist, and returns the registered stream. cfg may be nil
// or sparse.
func (c *StreamController) Register(ctx context.Context, systemId int64, name string, cfg *stream.StreamConfig) (*stream.Stream, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
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

	registered, err := c.datastore.Register(ctx, toStreamConfigRow(systemId, name, cfg), common.ProcessIdentity)
	if err != nil {
		return nil, err
	}

	return toStream(registered)
}

// Rename moves the stream under oldName to newName.
// Returns (nil, nil) if no stream is registered under oldName
// ErrStreamNameTaken if newName is already registered.
func (c *StreamController) Rename(ctx context.Context, oldName string, newName string) (*stream.Stream, error) {
	if oldName == "" {
		return nil, errors.New("oldName is required")
	}
	if !stream.SlugPattern.MatchString(newName) {
		return nil, fmt.Errorf("new name must match %s, got %q", stream.SlugPattern, newName)
	}
	if newName == oldName {
		return nil, errors.New("new name matches the current name -- nothing to rename")
	}

	renamed, err := c.datastore.Rename(ctx, oldName, newName, common.ProcessIdentity)
	if err != nil || renamed == nil {
		return nil, err
	}
	return toStream(renamed)
}

// Delete drains and drops the stream's tables, then removes its rows.
// name is only for error and log text.
func (c *StreamController) Delete(ctx context.Context, streamId int64, name string) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if name == "" {
		return errors.New("name is required")
	}

	return c.datastore.Delete(ctx, streamId, name)
}

// IsEmpty reports whether the stream's log holds any row at all.
func (c *StreamController) IsEmpty(ctx context.Context, streamId int64) (bool, error) {
	if streamId <= 0 {
		return false, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	return c.datastore.IsEmpty(ctx, streamId)
}
