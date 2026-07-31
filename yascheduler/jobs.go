package yascheduler

import (
	"context"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/google/uuid"
)

// UpsertJob creates or updates the job identified by spec.Key on the
// scheduler and returns the job UUID this client minted for it. The client
// must hold a registered connection; use AwaitConnected first when racing
// startup. Signature fields left empty on spec.Function are stamped from
// this client's registry when the target function is registered locally,
// and a backfill mode of BackfillModeInherit is stamped with this
// client's DefaultBackfill before sending.
func (c *Client) UpsertJob(
	ctx context.Context,
	spec *JobSpec,
) (protocol.JobUUID, yaerrors.Error) {
	if spec == nil {
		return protocol.JobUUID{}, yaerrors.FromError(
			http.StatusBadRequest,
			ErrNilJobSpec,
			logTag+" upsert job",
		)
	}

	if spec.Key == "" {
		return protocol.JobUUID{}, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyJobKey,
			logTag+" upsert job",
		)
	}

	if spec.Function.Name == "" {
		return protocol.JobUUID{}, yaerrors.FromError(
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
			return protocol.JobUUID{}, encodeErr.Wrap(logTag + " upsert job: encode args")
		}

		args = encoded
	}

	jobUUID := protocol.JobUUID(uuid.New())

	correlationID := c.nextCorrelation()

	waiter, err := c.registerPending(correlationID)
	if err != nil {
		return protocol.JobUUID{}, err.Wrap(logTag + " upsert job")
	}

	if err = c.enqueueFrame(correlationID, &protocol.JobUpsert{
		JobUUID:      jobUUID,
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

		return protocol.JobUUID{}, err.Wrap(logTag + " upsert job")
	}

	select {
	case <-ctx.Done():
		c.unregisterPending(correlationID)

		return protocol.JobUUID{}, yaerrors.FromError(
			http.StatusServiceUnavailable,
			ctx.Err(),
			logTag+" upsert job",
		)
	case ack, open := <-waiter:
		if !open {
			return protocol.JobUUID{}, yaerrors.FromError(
				http.StatusServiceUnavailable,
				ErrConnectionClosed,
				logTag+" upsert job",
			)
		}

		if !ack.Accepted {
			return protocol.JobUUID{}, yaerrors.FromError(
				http.StatusBadRequest,
				ErrUpsertRejected,
				logTag+" upsert job: "+wireErrorText(ack.Error),
			)
		}

		return ack.JobUUID, nil
	}
}
