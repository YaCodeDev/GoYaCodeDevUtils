package engine

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

func (e *engine) HandleLabelUpdate(
	ctx context.Context,
	instanceID protocol.InstanceID,
	update *protocol.LabelUpdate,
) (ack *protocol.LabelUpdateAck) {
	active, err := e.registry.UpdateLabels(instanceID, update.Announce, update.Withdraw)
	if err != nil {
		e.metrics.LabelUpdatesRejected.Add(1)

		e.log.WithFields(map[string]any{
			logFieldInstanceID: string(instanceID),
			"announced":        len(update.Announce),
			"withdrawn":        len(update.Withdraw),
		}).Warnf(logTag+" label update refused: %v", err)

		return &protocol.LabelUpdateAck{
			Accepted:    false,
			ActiveCount: uint32(active),
			Error: &protocol.WireError{
				Code:    protocol.ErrorCodeLabelRejected,
				Message: err.UnwrapLastError(),
			},
		}
	}

	if len(update.Withdraw) > 0 {
		e.reportWithdrawnUnderLoad(ctx, instanceID, update.Withdraw)
	}

	return &protocol.LabelUpdateAck{Accepted: true, ActiveCount: uint32(active)}
}

// reportWithdrawnUnderLoad records that a connection dropped routing labels
// while it still owed results. The attempts it holds run to completion:
// labels bind at dispatch, so cancelling them here would turn a routing
// decision into a duplicate-execution race against the redispatch. A moved
// resource surfaces as a retryable function failure, and the retry routes to
// the new holder on its own. This is visibility, not intervention.
func (e *engine) reportWithdrawnUnderLoad(
	ctx context.Context,
	instanceID protocol.InstanceID,
	withdrawn []protocol.Label,
) {
	open, err := e.attempts.AttemptsOnInstance(
		ctx,
		instanceID,
		store.AttemptDispatched,
		store.AttemptAccepted,
	)
	if err != nil {
		e.log.Errorf(logTag+" withdrawn label attempt lookup failed: %v", err)

		return
	}

	if len(open) == 0 {
		return
	}

	e.metrics.LabelWithdrawnInFlight.Add(1)

	e.log.WithFields(map[string]any{
		logFieldInstanceID: string(instanceID),
		"withdrawn":        len(withdrawn),
		"in_flight":        len(open),
	}).Warn(logTag + " labels withdrawn while attempts are still in flight")
}
