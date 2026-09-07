package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/producer"
)

// AlertController is the alert domain's write path: it records what a run
// found to the __system.alerts topic and logs status changes.
type AlertController struct {
	Logger logging.Logger

	ds     *datastore.PostgresDatastore
	alerts *producer.ProducerInstance[alert.Alert]
	heads  *compactioncontroller.CompactionController
	repeat time.Duration
}

// repeat is the worker's repeat interval, clamped below alert retention.
func NewAlertController(ctx context.Context, alerts *producer.ProducerInstance[alert.Alert], ds *datastore.PostgresDatastore, heads *compactioncontroller.CompactionController, repeat time.Duration, logger logging.Logger) (*AlertController, error) {
	if alerts == nil {
		return nil, errors.New("alert producer instance must not be nil")
	}
	if ds == nil {
		return nil, errors.New("ds must not be nil")
	}
	if heads == nil {
		return nil, errors.New("compaction controller must not be nil")
	}
	if repeat <= 0 {
		return nil, fmt.Errorf("repeat must be > 0, got %v", repeat)
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	// alert repeat needs to be less than retention ttl otherwise could sweep
	// alert head and fake repeat early
	retention := alerts.Topic.RetentionTTL
	if retention > 0 && repeat >= retention {
		clamped := retention / 2
		logger.WarnContext(ctx, "alert repeat interval at or above the alerts topic's retention -- clamped",
			"repeat", repeat, "retention", retention, "clamped", clamped)
		repeat = clamped
	}
	return &AlertController{Logger: logger, ds: ds, alerts: alerts, heads: heads, repeat: repeat}, nil
}
