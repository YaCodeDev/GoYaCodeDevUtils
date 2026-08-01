package yascheduler

import (
	"context"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/google/uuid"
)

// buildJobUpsert turns one job spec into the upsert message either
// scheduler implementation submits, validating the spec and minting the
// job identity. Signature fields left empty on spec.Function are stamped
// from the local registry when the target function is registered there,
// and a backfill mode of BackfillModeInherit is stamped with the given
// default before sending.
func buildJobUpsert(
	spec *JobSpec,
	registry *Registry,
	defaultExecutorType protocol.ExecutorType,
	defaultBackfill protocol.BackfillMode,
) (*protocol.JobUpsert, yaerrors.Error) {
	if spec == nil {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrNilJobSpec,
			logTag+" build job upsert",
		)
	}

	if spec.Key == "" {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyJobKey,
			logTag+" build job upsert",
		)
	}

	if spec.Function.Name == "" {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyFunctionName,
			logTag+" build job upsert",
		)
	}

	executorType := spec.ExecutorType
	if executorType == "" {
		executorType = defaultExecutorType
	}

	function := spec.Function

	if executorType == defaultExecutorType {
		if local, found := registry.lookup(function.Name, function.Version); found {
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
		backfill.Mode = defaultBackfill
	}

	var args []byte

	if spec.Args != nil {
		encoded, encodeErr := yaencoding.EncodeMessagePack(spec.Args)
		if encodeErr != nil {
			return nil, encodeErr.Wrap(logTag + " build job upsert: encode args")
		}

		args = encoded
	}

	return &protocol.JobUpsert{
		JobUUID:      protocol.JobUUID(uuid.New()),
		JobKey:       spec.Key,
		ExecutorType: executorType,
		Function:     function,
		Args:         args,
		Schedule:     spec.Schedule,
		Enabled:      !spec.Disabled,
		Backfill:     backfill,
		Retry:        spec.Retry,
		Overlap:      spec.Overlap,
		Pin:          spec.Pin,
	}, nil
}

// UpsertJob creates or updates the job identified by spec.Key on the
// scheduler and returns the job UUID this client minted for it. The client
// must hold a registered connection; use AwaitReady first when racing
// startup. Signature fields left empty on spec.Function are stamped from
// this client's registry when the target function is registered locally,
// and a backfill mode of BackfillModeInherit is stamped with this
// client's DefaultBackfill before sending.
func (c *Client) UpsertJob(
	ctx context.Context,
	spec *JobSpec,
) (protocol.JobUUID, yaerrors.Error) {
	upsert, err := buildJobUpsert(spec, c.registry, c.cfg.ExecutorType, c.cfg.DefaultBackfill)
	if err != nil {
		return protocol.JobUUID{}, err.Wrap(logTag + " upsert job")
	}

	correlationID := c.nextCorrelation()

	waiter, err := c.registerPending(correlationID)
	if err != nil {
		return protocol.JobUUID{}, err.Wrap(logTag + " upsert job")
	}

	if err = c.enqueueFrame(correlationID, upsert); err != nil {
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
