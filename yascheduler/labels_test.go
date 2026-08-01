package yascheduler_test

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	labelShardA protocol.Label = "shard-a"
	labelShardB protocol.Label = "shard-b"
	labelShardC protocol.Label = "shard-c"

	labelAckActiveOne uint32 = 1

	labelBlockProbe = 100 * time.Millisecond
)

// ackAnnounce drives one accepted announce round trip on conn: it waits
// for the client to attach the registered connection, starts the
// announce, consumes the LabelUpdate frame, acknowledges it, and waits
// for the announce call to return.
func ackAnnounce(
	t *testing.T,
	client *yascheduler.Client,
	conn net.Conn,
	label protocol.Label,
) {
	t.Helper()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer readyCancel()

	if err := client.AwaitReady(readyCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	outcome := make(chan error, 1)

	go func() {
		announceCtx, announceCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer announceCancel()

		outcome <- client.AnnounceLabels(announceCtx, label)
	}()

	header, update := waitForMessage[*protocol.LabelUpdate](t, conn)

	if !slices.Contains(update.Announce, label) {
		t.Fatalf("announce labels = %v, want %v", update.Announce, label)
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.LabelUpdateAck{
		Accepted:    true,
		ActiveCount: labelAckActiveOne,
	})

	select {
	case err := <-outcome:
		if err != nil {
			t.Fatalf("AnnounceLabels failed after an accepted ack: %v", err)
		}
	case <-time.After(testReadTimeout):
		t.Fatal("AnnounceLabels never returned after the ack")
	}
}

func TestClientAnnounceWaitsForAck(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	running := startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	outcome := make(chan error, 1)

	go func() {
		announceCtx, announceCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer announceCancel()

		outcome <- running.client.AnnounceLabels(announceCtx, labelShardA)
	}()

	header, update := waitForMessage[*protocol.LabelUpdate](t, conn)

	if !slices.Contains(update.Announce, labelShardA) {
		t.Fatalf("announce labels = %v, want %v", update.Announce, labelShardA)
	}

	if len(update.Withdraw) != 0 {
		t.Fatalf("withdraw labels = %v, want none", update.Withdraw)
	}

	select {
	case err := <-outcome:
		t.Fatalf("AnnounceLabels returned before the ack: %v", err)
	case <-time.After(labelBlockProbe):
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.LabelUpdateAck{
		Accepted:    true,
		ActiveCount: labelAckActiveOne,
	})

	select {
	case err := <-outcome:
		if err != nil {
			t.Fatalf("AnnounceLabels failed after an accepted ack: %v", err)
		}
	case <-time.After(testReadTimeout):
		t.Fatal("AnnounceLabels never returned after the ack")
	}
}

func TestClientAnnounceRejectedKeepsLabelSet(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	running := startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	outcome := make(chan error, 1)

	go func() {
		announceCtx, announceCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer announceCancel()

		outcome <- running.client.AnnounceLabels(announceCtx, labelShardA)
	}()

	header, _ := waitForMessage[*protocol.LabelUpdate](t, conn)

	writeMessage(t, conn, header.CorrelationID, &protocol.LabelUpdateAck{
		Accepted: false,
		Error: &protocol.WireError{
			Code:    protocol.ErrorCodeLabelRejected,
			Message: "too many labels",
		},
	})

	select {
	case err := <-outcome:
		if !errors.Is(err, yascheduler.ErrLabelUpdateRejected) {
			t.Fatalf("error = %v, want ErrLabelUpdateRejected", err)
		}
	case <-time.After(testReadTimeout):
		t.Fatal("AnnounceLabels never returned after the rejection")
	}

	_ = conn.Close()

	_, register := acceptAndRegister(t, fs)

	if len(register.Labels) != 0 {
		t.Fatalf(
			"labels = %v after a rejected announce, want none: a rejection "+
				"must leave the local set untouched",
			register.Labels,
		)
	}
}

func TestClientReplaysLabelsOnReconnect(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	client, err := yascheduler.New(testClientConfig(fs), yascheduler.NewRegistry(), nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	disconnectedCtx, disconnectedCancel := context.WithTimeout(
		context.Background(),
		testReadTimeout,
	)
	defer disconnectedCancel()

	if announceErr := client.AnnounceLabels(disconnectedCtx, labelShardA); announceErr != nil {
		t.Fatalf("disconnected AnnounceLabels failed: %v", announceErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		if runErr := client.Run(ctx); runErr != nil {
			t.Errorf("Run returned error: %v", runErr)
		}
	}()

	t.Cleanup(func() {
		cancel()

		select {
		case <-done:
		case <-time.After(testRunStopTimeout):
			t.Fatal("client Run did not stop in time")
		}
	})

	firstConn, firstRegister := acceptAndRegister(t, fs)

	if !slices.Contains(firstRegister.Labels, labelShardA) {
		t.Fatalf(
			"first register labels = %v, want %v announced while disconnected",
			firstRegister.Labels,
			labelShardA,
		)
	}

	ackAnnounce(t, client, firstConn, labelShardB)

	_ = firstConn.Close()

	secondConn, secondRegister := acceptAndRegister(t, fs)

	for _, label := range []protocol.Label{labelShardA, labelShardB} {
		if !slices.Contains(secondRegister.Labels, label) {
			t.Fatalf("second register labels = %v, want %v", secondRegister.Labels, label)
		}
	}

	ackAnnounce(t, client, secondConn, labelShardC)

	_ = secondConn.Close()

	_, thirdRegister := acceptAndRegister(t, fs)

	for _, label := range []protocol.Label{labelShardA, labelShardB, labelShardC} {
		if !slices.Contains(thirdRegister.Labels, label) {
			t.Fatalf(
				"third register labels = %v, want %v: an announce made after "+
					"a reconnect must replay like any other",
				thirdRegister.Labels,
				label,
			)
		}
	}
}
