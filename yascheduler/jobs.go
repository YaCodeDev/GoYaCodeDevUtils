package yascheduler

import (
	"context"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// UpsertJob creates or updates the job identified by spec.Key on the
// scheduler and returns the scheduler-minted job ID. The client must
// hold a registered connection; use AwaitConnected first when racing
// startup. Signature fields left empty on spec.Function are stamped from
// this client's registry when the target function is registered locally,
// and a backfill mode of BackfillModeInherit is stamped with this
// client's DefaultBackfill before sending.
func (c *Client) UpsertJob(
	ctx context.Context,
	spec *JobSpec,
) (protocol.JobID, yaerrors.Error) {
	if spec == nil {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			ErrNilJobSpec,
			logTag+" upsert job",
		)
	}

	if spec.Key == "" {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyJobKey,
			logTag+" upsert job",
		)
	}

	if spec.Function.Name == "" {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyFunctionName,
			logTag+" upsert job",
		)
	}

	executorType := spec.ExecutorType
	if executorType == "" {
		executorType = c.cfg.ExecutorType
	}

	function := spec.Function

	if executorType == c.cfg.ExecutorType {
		if local, found := c.registry.lookup(function.Name, function.Version); found {
			if function.InputSignature == "" {
				function.InputSignature = local.spec.InputSignature
			}

			if function.OutputSignature == "" {
				function.OutputSignature = local.spec.OutputSignature
			}
		}
	}

	backfill := spec.Backfill
	if backfill.Mode == protocol.BackfillModeInherit {
		backfill.Mode = c.cfg.DefaultBackfill
	}

	var args []byte

	if spec.Args != nil {
		encoded, encodeErr := yaencoding.EncodeMessagePack(spec.Args)
		if encodeErr != nil {
			return 0, encodeErr.Wrap(logTag + " upsert job: encode args")
		}

		args = encoded
	}

	correlationID := c.nextCorrelation()

	waiter, err := c.registerPending(correlationID)
	if err != nil {
		return 0, err.Wrap(logTag + " upsert job")
	}

	if err = c.enqueueFrame(correlationID, &protocol.JobUpsert{
		JobKey:       spec.Key,
		ExecutorType: executorType,
		Function:     function,
		Args:         args,
		Schedule:     spec.Schedule,
		Enabled:      !spec.Disabled,
		Backfill:     backfill,
		Retry:        spec.Retry,
		Overlap:      spec.Overlap,
	}); err != nil {
		c.unregisterPending(correlationID)

		return 0, err.Wrap(logTag + " upsert job")
	}

	select {
	case <-ctx.Done():
		c.unregisterPending(correlationID)

		return 0, yaerrors.FromError(
			http.StatusServiceUnavailable,
			ctx.Err(),
			logTag+" upsert job",
		)
	case ack, open := <-waiter:
		if !open {
			return 0, yaerrors.FromError(
				http.StatusServiceUnavailable,
				ErrConnectionClosed,
				logTag+" upsert job",
			)
		}

		if !ack.Accepted {
			return 0, yaerrors.FromError(
				http.StatusBadRequest,
				ErrUpsertRejected,
				logTag+" upsert job: "+wireErrorText(ack.Error),
			)
		}

		return ack.JobID, nil
	}
}
