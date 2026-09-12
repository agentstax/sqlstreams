package cli

import (
	"encoding/json"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newConsumerWorkerCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Inspect a consumer's declared workers",
		Long: `Consumer workers are declared at Consumer(name).Register. Their stored config
comes from ConsumerConfig. Running instances refresh it at ConfigRefreshInterval.
Session settings such as ConsumeOptions.ClaimPollRate are not stored worker config.`,
	}

	cmd.AddCommand(newConsumerWorkerListCmd(g))

	return cmd
}

// messageFieldKey is one field of the message document: the dotted path
// worker list prints, and how the CLI reads it back.
type messageFieldKey struct {
	path string
	read func(options *common.MessageOptions) string
}

var messageFieldKeys = []messageFieldKey{
	{
		path: "message.concurrency",
		read: func(options *common.MessageOptions) string {
			return string(options.Concurrency)
		},
	},
	{
		path: "message.timeout",
		read: func(options *common.MessageOptions) string {
			return durationCell(options.Timeout)
		},
	},
	{
		path: "message.retry.max_retries",
		read: func(options *common.MessageOptions) string {
			return intCell(options.Retry.MaxRetries)
		},
	},
	{
		path: "message.retry.base_delay",
		read: func(options *common.MessageOptions) string {
			return durationCell(options.Retry.BaseDelay)
		},
	},
	{
		path: "message.retry.max_delay",
		read: func(options *common.MessageOptions) string {
			return durationCell(options.Retry.MaxDelay)
		},
	},
	{
		path: "message.retry.exponent",
		read: func(options *common.MessageOptions) string {
			return intCell(options.Retry.Exponent)
		},
	},
}

// decodeMessageOptions decodes a worker row's stored message document.
func decodeMessageOptions(value any) (*common.MessageOptions, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var options common.MessageOptions
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil, err
	}
	// Retry may be absent from the document; field reads assume it isn't
	if options.Retry == nil {
		options.Retry = &common.RetryPolicy{}
	}
	return &options, nil
}

// consumerError maps a consumer read failure to CLI output.
func consumerError(streamName string, consumerName string, err error) error {
	switch {
	case errors.Is(err, consume.ErrConsumerNotFound):
		return failOp("consumer %q not found on stream %q", consumerName, streamName)
	case errors.Is(err, stream.ErrStreamNotFound):
		return errStreamNotFound(streamName)
	default:
		return translateAdminError(err)
	}
}
