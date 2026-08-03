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
// job identity. An empty spec.Key means an RPC-style one-shot: the minted
// job UUID doubles as the key, so the job is guaranteed fresh and cannot
// collide with an in-flight upsert of the same key. Signature fields left
// empty on spec.Function are stamped from the local registry when the
// target function is registered there, and a backfill mode of
// BackfillModeInherit is stamped with the given default before sending.
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

	jobUUID := protocol.JobUUID(uuid.New())

	jobKey := spec.Key
	if jobKey == "" {
		jobKey = jobUUID.String()
	}

	return &protocol.JobUpsert{
		JobUUID:      jobUUID,
		JobKey:       jobKey,
		ExecutorType: executorType,
		Function:     function,
		Args:         args,
		Schedule:     spec.Schedule,
		Enabled:      !spec.Disabled,
		Backfill:     backfill,
		Retry:        spec.Retry,
		Overlap:      spec.Overlap,
		Pin:          spec.Pin,
		ResultMode:   spec.ResultMode,
	}, nil
}

// UpsertJob creates or updates the job identified by spec.Key within its
// executor type on the scheduler and returns the submission handle for
// it; an empty spec.Key
// submits an RPC-style one-shot keyed by the minted job UUID. Under
// ResultModeDeliver the result waiter is registered before the upsert is
// sent, so a result settling before this call returns is never missed.
// The client must hold a registered connection; use AwaitReady first when
// racing startup. Signature fields left empty on spec.Function are
// stamped from this client's registry when the target function is
// registered locally, and a backfill mode of BackfillModeInherit is
// stamped with this client's DefaultBackfill before sending.
func (c *Client) UpsertJob(
	ctx context.Context,
	spec *JobSpec,
) (*Submission, yaerrors.Error) {
	upsert, err := buildJobUpsert(spec, c.registry, c.cfg.ExecutorType, c.cfg.DefaultBackfill)
	if err != nil {
		return nil, err.Wrap(logTag + " upsert job")
	}

	submission := c.results.open(upsert.JobUUID, upsert.ResultMode)

	correlationID := c.nextCorrelation()

	waiter, err := c.registerPending(correlationID)
	if err != nil {
		submission.Close()

		return nil, err.Wrap(logTag + " upsert job")
	}

	if err = c.enqueueFrame(correlationID, upsert); err != nil {
		c.unregisterPending(correlationID)
		submission.Close()

		return nil, err.Wrap(logTag + " upsert job")
	}

	select {
	case <-ctx.Done():
		c.unregisterPending(correlationID)
		submission.Close()

		return nil, yaerrors.FromError(
			http.StatusServiceUnavailable,
			ctx.Err(),
			logTag+" upsert job",
		)
	case reply, open := <-waiter:
		if !open {
			submission.Close()

			return nil, yaerrors.FromError(
				http.StatusServiceUnavailable,
				ErrConnectionClosed,
				logTag+" upsert job",
			)
		}

		ack := reply.upsertAck
		if ack == nil {
			submission.Close()

			return nil, yaerrors.FromError(
				http.StatusBadGateway,
				ErrUnexpectedMessage,
				logTag+" upsert job",
			)
		}

		if !ack.Accepted {
			submission.Close()

			return nil, yaerrors.FromError(
				http.StatusBadRequest,
				ErrUpsertRejected,
				logTag+" upsert job: "+wireErrorText(ack.Error),
			)
		}

		submission.adopt(ack.JobUUID)

		return submission, nil
	}
}
