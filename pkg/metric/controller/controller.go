package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	compactioncontroller "github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric/controller/datastore"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
)

// MetricController owns live snapshots and retained measurement reads.
type MetricController struct {
	Logger logging.Logger

	datastore *datastore.MetricDatastore
	heads     *compactioncontroller.CompactionController
	streams   *streamcontroller.StreamController
}

func NewMetricsController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*MetricController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	metricDatastore, err := datastore.NewMetricsDatastore(ds, logger)
	if err != nil {
		return nil, err
	}
	heads, err := compactioncontroller.NewCompactionController(ds, logger)
	if err != nil {
		return nil, err
	}
	streams, err := streamcontroller.NewStreamController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &MetricController{
		Logger:    logger,
		datastore: metricDatastore,
		heads:     heads,
		streams:   streams,
	}, nil
}
