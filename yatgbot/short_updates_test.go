package yatgbot

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"
)

func TestShortUpdateNormalizerConvertsShortMessage(t *testing.T) {
	dispatcher := tg.NewUpdateDispatcher()

	var got *tg.UpdateNewMessage
	dispatcher.OnNewMessage(
		func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
			got = update

			return nil
		},
	)

	handler := newShortUpdateNormalizer(dispatcher)
	err := handler.Handle(context.Background(), &tg.UpdateShortMessage{
		ID:       10,
		UserID:   42,
		Message:  "/help",
		Pts:      100,
		PtsCount: 1,
		Date:     123,
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBotCommand{Offset: 0, Length: 5},
		},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if got == nil {
		t.Fatal("short update was not dispatched as UpdateNewMessage")
	}

	message, ok := got.Message.(*tg.Message)
	if !ok {
		t.Fatalf("message type = %T, want *tg.Message", got.Message)
	}

	if got.Pts != 100 || got.PtsCount != 1 {
		t.Fatalf("pts = %d/%d, want 100/1", got.Pts, got.PtsCount)
	}

	if message.Message != "/help" {
		t.Fatalf("message text = %q, want /help", message.Message)
	}

	if peer, ok := message.PeerID.(*tg.PeerUser); !ok || peer.UserID != 42 {
		t.Fatalf("peer = %#v, want PeerUser 42", message.PeerID)
	}

	if from, ok := message.FromID.(*tg.PeerUser); !ok || from.UserID != 42 {
		t.Fatalf("from = %#v, want PeerUser 42", message.FromID)
	}
}

func TestShortUpdateNormalizerKeepsOutgoingShortMessageIgnored(t *testing.T) {
	dispatcher := tg.NewUpdateDispatcher()

	called := false
	dispatcher.OnNewMessage(
		func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
			called = true

			return nil
		},
	)

	handler := newShortUpdateNormalizer(dispatcher)
	err := handler.Handle(context.Background(), &tg.UpdateShortMessage{
		Out:     true,
		ID:      10,
		UserID:  42,
		Message: "outgoing",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if called {
		t.Fatal("outgoing short message should stay ignored")
	}
}
