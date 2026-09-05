package consumer

import (
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consume/exceptionconsumer"
	"github.com/agentstax/vulkan/pkg/consume/messageconsumer"
	"github.com/agentstax/vulkan/pkg/worker"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
)

func toMessageConsumerWorkerConfig(declared *ConsumerConfig) *workercontroller.WorkerConfig {
	return &workercontroller.WorkerConfig{
		Metadata: &messageconsumer.MessageConsumerMetadata{
			Message:                 declared.Message,
			MessageMin:              declared.MessageMin,
			MessageMax:              declared.MessageMax,
			ConcurrencyOverride:     declared.ConcurrencyOverride,
			ExceptionInitialBackoff: declared.ExceptionInitialBackoff,
			MaxRangeReclaims:        declared.MaxRangeReclaims,
		},
		TargetInstances: worker.NoInstanceTarget,
	}
}

func toMessageConsumerConfig(cfg *ConsumerConfig, options *ConsumeOptions, retry *common.RetryPolicy, logger logging.Logger) *messageconsumer.MessageConsumerConfig {
	return &messageconsumer.MessageConsumerConfig{
		BatchLimit:              options.BatchLimit,
		QueueSize:               options.QueueSize,
		MessageConcurrency:      options.MessageConcurrency,
		MaxRangeReclaims:        cfg.MaxRangeReclaims,
		ClaimPollRate:           options.ClaimPollRate,
		QueueMargin:             options.QueueMargin,
		RecordMargin:            options.RecordMargin,
		TimeoutGrace:            options.TimeoutGrace,
		SlowDispatchThreshold:   options.SlowDispatchThreshold,
		ExceptionInitialBackoff: cfg.ExceptionInitialBackoff,
		ShutdownTimeout:         options.ShutdownTimeout,
		InstanceTTL:             options.InstanceTTL,
		ConfigRefreshInterval:   options.ConfigRefreshInterval,
		Message:                 cfg.Message,
		MessageMin:              cfg.MessageMin,
		MessageMax:              cfg.MessageMax,
		ConcurrencyOverride:     cfg.ConcurrencyOverride,
		Logger:                  logger,
		Retry:                   retry,
	}
}

func toExceptionConsumerConfig(cfg *ConsumerConfig, options *ConsumeOptions, retry *common.RetryPolicy, logger logging.Logger) *exceptionconsumer.ExceptionConsumerConfig {
	return &exceptionconsumer.ExceptionConsumerConfig{
		BatchLimit:            options.BatchLimit,
		ClaimPollRate:         options.ClaimPollRate,
		QueueMargin:           options.QueueMargin,
		RecordMargin:          options.RecordMargin,
		TimeoutGrace:          options.TimeoutGrace,
		SlowDispatchThreshold: options.SlowDispatchThreshold,
		InstanceTTL:           options.InstanceTTL,
		ConfigRefreshInterval: options.ConfigRefreshInterval,
		Message:               cfg.Message,
		MessageMin:            cfg.MessageMin,
		MessageMax:            cfg.MessageMax,
		ConcurrencyOverride:   cfg.ConcurrencyOverride,
		Logger:                logger,
		Retry:                 retry,
	}
}

func toExceptionConsumerWorkerConfig(declared *ConsumerConfig) *workercontroller.WorkerConfig {
	return &workercontroller.WorkerConfig{
		Metadata: &exceptionconsumer.ExceptionConsumerMetadata{
			Message:             declared.Message,
			MessageMin:          declared.MessageMin,
			MessageMax:          declared.MessageMax,
			ConcurrencyOverride: declared.ConcurrencyOverride,
		},
		TargetInstances: worker.NoInstanceTarget,
	}
}
