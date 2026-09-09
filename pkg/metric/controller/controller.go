package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/metric/controller/datastore"
	topiccontroller "github.com/agentstax/vulkan/pkg/topic/controller"
)

// MetricController owns live snapshots and retained measurement reads.
type MetricController struct {
	Logger logging.Logger

	datastore *datastore.MetricDatastore
	heads     *compactioncontroller.CompactionController
	topics    *topiccontroller.TopicController
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
	topics, err := topiccontroller.NewTopicController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &MetricController{
		Logger:    logger,
		datastore: metricDatastore,
		heads:     heads,
		topics:    topics,
	}, nil
}
