package yatgbot

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gotd/td/tg"
)

func TestUpdateBuilders(t *testing.T) {
	t.Parallel()

	dispatcher := &Dispatcher{
		BotUser:   &tg.User{ID: 99},
		Localizer: yalocales.NewLocalizer("en", true),
		Log:       yalogger.NewBaseLogger(nil).NewLogger(),
	}
	ent := testEntities()

	t.Run("buildNewMessageUpdateData", func(t *testing.T) {
		t.Parallel()

		deps, ok := dispatcher.buildNewMessageUpdateData(ent, &tg.UpdateNewMessage{
			Message: &tg.Message{
				PeerID:  &tg.PeerUser{UserID: 1},
				FromID:  &tg.PeerUser{UserID: 1},
				Message: "hello",
			},
		})
		if !ok || deps.userID != 1 || deps.chatID != 1 {
			t.Fatalf("buildNewMessageUpdateData() = (%+v, %v), want userID=1 chatID=1", deps, ok)
		}

		deps, ok = dispatcher.buildNewMessageUpdateData(tg.Entities{Short: true}, &tg.UpdateNewMessage{
			Message: &tg.Message{
				PeerID:  &tg.PeerUser{UserID: 404},
				FromID:  &tg.PeerUser{UserID: 404},
				Message: "/help",
			},
		})
		if !ok || deps.userID != 404 || deps.chatID != 404 {
			t.Fatalf("buildNewMessageUpdateData(short) = (%+v, %v), want userID=404 chatID=404", deps, ok)
		}

		if peer, ok := deps.inputPeer.(*tg.InputPeerUser); !ok || peer.UserID != 404 || peer.AccessHash != 0 {
			t.Fatalf("buildNewMessageUpdateData(short) peer = %#v, want userID=404 accessHash=0", deps.inputPeer)
		}

		deps, ok = dispatcher.buildNewMessageUpdateData(ent, &tg.UpdateNewMessage{
			Message: &tg.MessageService{
				PeerID: &tg.PeerChat{ChatID: 99},
				FromID: &tg.PeerUser{UserID: 2},
				Action: &tg.MessageActionChatCreate{Title: "room"},
			},
		})
		if !ok || deps.userID != 2 || deps.chatID != 99 {
			t.Fatalf("buildNewMessageUpdateData(service) = (%+v, %v), want userID=2 chatID=99", deps, ok)
		}

		if _, ok := dispatcher.buildNewMessageUpdateData(ent, &tg.UpdateNewMessage{
			Message: &tg.Message{
				PeerID: &tg.PeerUser{UserID: 1},
				FromID: &tg.PeerUser{UserID: 99},
			},
		}); ok {
			t.Fatal("buildNewMessageUpdateData() unexpectedly accepted bot-originated message")
		}

		if _, ok := dispatcher.buildNewMessageUpdateData(ent, &tg.UpdateNewMessage{}); ok {
			t.Fatal("buildNewMessageUpdateData() unexpectedly accepted empty update")
		}
	})

	t.Run("buildNewChannelMessageUpdateData", func(t *testing.T) {
		t.Parallel()

		deps, ok := dispatcher.buildNewChannelMessageUpdateData(ent, &tg.UpdateNewChannelMessage{
			Message: &tg.Message{
				PeerID: &tg.PeerChannel{ChannelID: 7},
			},
		})
		if !ok || deps.chatID != 7 || deps.userID != 0 {
			t.Fatalf("buildNewChannelMessageUpdateData() = (%+v, %v), want chatID=7 userID=0", deps, ok)
		}

		deps, ok = dispatcher.buildNewChannelMessageUpdateData(ent, &tg.UpdateNewChannelMessage{
			Message: &tg.MessageService{
				PeerID: &tg.PeerChannel{ChannelID: 7},
				Action: &tg.MessageActionChatCreate{Title: "room"},
			},
		})
		if !ok || deps.chatID != 7 {
			t.Fatalf("buildNewChannelMessageUpdateData(service) = (%+v, %v), want chatID=7", deps, ok)
		}

		if _, ok := dispatcher.buildNewChannelMessageUpdateData(ent, &tg.UpdateNewChannelMessage{}); ok {
			t.Fatal("buildNewChannelMessageUpdateData() unexpectedly accepted empty update")
		}
	})

	t.Run("buildEditMessageUpdateData", func(t *testing.T) {
		t.Parallel()

		deps, ok := dispatcher.buildEditMessageUpdateData(ent, &tg.UpdateEditMessage{
			Message: &tg.Message{
				PeerID: &tg.PeerChat{ChatID: 99},
				FromID: &tg.PeerUser{UserID: 1},
			},
		})
		if !ok || deps.userID != 1 || deps.chatID != 99 {
			t.Fatalf("buildEditMessageUpdateData() = (%+v, %v), want userID=1 chatID=99", deps, ok)
		}

		if _, ok := dispatcher.buildEditMessageUpdateData(ent, &tg.UpdateEditMessage{}); ok {
			t.Fatal("buildEditMessageUpdateData() unexpectedly accepted empty update")
		}
	})

	t.Run("buildEditChannelMessageUpdateData", func(t *testing.T) {
		t.Parallel()

		deps, ok := dispatcher.buildEditChannelMessageUpdateData(ent, &tg.UpdateEditChannelMessage{
			Message: &tg.Message{
				PeerID: &tg.PeerChannel{ChannelID: 7},
			},
		})
		if !ok || deps.chatID != 7 {
			t.Fatalf("buildEditChannelMessageUpdateData() = (%+v, %v), want chatID=7", deps, ok)
		}

		if _, ok := dispatcher.buildEditChannelMessageUpdateData(ent, &tg.UpdateEditChannelMessage{}); ok {
			t.Fatal("buildEditChannelMessageUpdateData() unexpectedly accepted empty update")
		}
	})

	t.Run("simple builders", func(t *testing.T) {
		t.Parallel()

		deps, ok := dispatcher.buildBotCallbackQueryUpdateData(ent, &tg.UpdateBotCallbackQuery{
			UserID: 1,
			Peer:   &tg.PeerChat{ChatID: 99},
		})
		if !ok || deps.userID != 1 || deps.chatID != 99 {
			t.Fatalf("buildBotCallbackQueryUpdateData() = (%+v, %v), want userID=1 chatID=99", deps, ok)
		}

		deps, ok = dispatcher.buildDeleteMessagesUpdateData(ent, &tg.UpdateDeleteMessages{})
		if !ok || deps.userID != 0 || deps.chatID != 0 {
			t.Fatalf("buildDeleteMessagesUpdateData() = (%+v, %v), want zero IDs", deps, ok)
		}

		deps, ok = dispatcher.buildDeleteChannelMessagesUpdateData(ent, &tg.UpdateDeleteChannelMessages{ChannelID: 7})
		if !ok || deps.chatID != 7 {
			t.Fatalf("buildDeleteChannelMessagesUpdateData() = (%+v, %v), want chatID=7", deps, ok)
		}

		deps, ok = dispatcher.buildChannelParticipantUpdateData(ent, &tg.UpdateChannelParticipant{
			UserID:    2,
			ChannelID: 7,
		})
		if !ok || deps.userID != 2 || deps.chatID != 7 {
			t.Fatalf("buildChannelParticipantUpdateData() = (%+v, %v), want userID=2 chatID=7", deps, ok)
		}

		deps, ok = dispatcher.buildBotMessageReactionsUpdateData(ent, &tg.UpdateBotMessageReactions{
			Peer: &tg.PeerChannel{ChannelID: 7},
		})
		if !ok || deps.chatID != 7 {
			t.Fatalf("buildBotMessageReactionsUpdateData() = (%+v, %v), want chatID=7", deps, ok)
		}

		deps, ok = dispatcher.buildBotPrecheckoutQueryUpdateData(ent, &tg.UpdateBotPrecheckoutQuery{UserID: 1})
		if !ok || deps.userID != 1 {
			t.Fatalf("buildBotPrecheckoutQueryUpdateData() = (%+v, %v), want userID=1", deps, ok)
		}

		deps, ok = dispatcher.buildBotInlineQueryUpdateData(ent, &tg.UpdateBotInlineQuery{UserID: 1})
		if !ok || deps.userID != 1 {
			t.Fatalf("buildBotInlineQueryUpdateData() = (%+v, %v), want userID=1", deps, ok)
		}
	})
}

func TestUserAndPeerScopedBuilders(t *testing.T) {
	t.Parallel()

	ent := testEntities()

	t.Run("buildUserScopedUpdateData uses first chat entry", func(t *testing.T) {
		t.Parallel()

		deps, ok := buildUserScopedUpdateData(ent, &tg.UpdateBotInlineQuery{UserID: 1}, 1)
		if !ok {
			t.Fatal("buildUserScopedUpdateData() unexpectedly returned false")
		}

		if deps.chatID != 99 {
			t.Fatalf("buildUserScopedUpdateData() chatID = %d, want 99", deps.chatID)
		}

		peer, ok := deps.inputPeer.(*tg.InputPeerChat)
		if !ok || peer.ChatID != 99 {
			t.Fatalf("buildUserScopedUpdateData() peer = %#v, want InputPeerChat{ChatID:99}", deps.inputPeer)
		}
	})

	t.Run("buildUserScopedUpdateData falls back to user peer", func(t *testing.T) {
		t.Parallel()

		entWithoutChats := ent
		entWithoutChats.Chats = nil

		deps, ok := buildUserScopedUpdateData(entWithoutChats, &tg.UpdateBotPrecheckoutQuery{UserID: 2}, 2)
		if !ok {
			t.Fatal("buildUserScopedUpdateData() unexpectedly returned false")
		}

		if deps.chatID != 2 {
			t.Fatalf("buildUserScopedUpdateData() chatID = %d, want 2", deps.chatID)
		}

		peer, ok := deps.inputPeer.(*tg.InputPeerUser)
		if !ok || peer.UserID != 2 || peer.AccessHash != 222 {
			t.Fatalf("buildUserScopedUpdateData() peer = %#v, want InputPeerUser{UserID:2, AccessHash:222}", deps.inputPeer)
		}
	})

	t.Run("buildUserScopedUpdateData returns false for missing user", func(t *testing.T) {
		t.Parallel()

		if _, ok := buildUserScopedUpdateData(ent, &tg.UpdateBotInlineQuery{UserID: 404}, 404); ok {
			t.Fatal("buildUserScopedUpdateData() unexpectedly resolved missing user")
		}
	})

	t.Run("buildPeerUpdateData and helper accessors", func(t *testing.T) {
		t.Parallel()

		upd := &tg.UpdateNewMessage{}
		deps, ok := buildPeerUpdateData(ent, upd, &tg.PeerChat{ChatID: 99}, &tg.PeerUser{UserID: 1})
		if !ok {
			t.Fatal("buildPeerUpdateData() unexpectedly returned false")
		}

		if deps.userID != 1 || deps.chatID != 99 || deps.update != upd {
			t.Fatalf("buildPeerUpdateData() = %+v, want userID=1 chatID=99", deps)
		}

		if peer, ok := makeInputPeer(&tg.PeerChannel{ChannelID: 7}, ent); !ok || peer == nil {
			t.Fatal("makeInputPeer() unexpectedly returned nil")
		}

		if got, ok := getChatID(&tg.PeerChannel{ChannelID: 7}, ent); !ok || got != 7 {
			t.Fatalf("getChatID() = %d (ok=%v), want 7,true", got, ok)
		}

		if got, ok := getUserID(&tg.PeerChat{ChatID: 99}, &tg.PeerUser{UserID: 2}); !ok || got != 2 {
			t.Fatalf("getUserID() = %d (ok=%v), want 2,true", got, ok)
		}
	})
}

func TestWrapAsyncBranches(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when builder rejects update", func(t *testing.T) {
		t.Parallel()

		var called atomic.Bool
		handler := wrapAsync(
			false,
			nil,
			func(_ tg.Entities, _ *tg.UpdateBotInlineQuery) (UpdateData, bool) {
				return UpdateData{}, false
			},
			func(_ context.Context, _ UpdateData) yaerrors.Error {
				called.Store(true)

				return nil
			},
		)

		if err := handler(context.Background(), tg.Entities{}, &tg.UpdateBotInlineQuery{}); err != nil {
			t.Fatalf("wrapAsync() error = %v", err)
		}

		if called.Load() {
			t.Fatal("wrapAsync() dispatched update even though builder rejected it")
		}
	})

	t.Run("dispatches synchronously when sync is enabled", func(t *testing.T) {
		t.Parallel()

		var called atomic.Bool
		handler := wrapAsync(
			true,
			nil,
			func(_ tg.Entities, _ *tg.UpdateBotInlineQuery) (UpdateData, bool) {
				return UpdateData{userID: 1}, true
			},
			func(_ context.Context, deps UpdateData) yaerrors.Error {
				called.Store(true)

				if deps.userID != 1 {
					t.Fatalf("dispatch userID = %d, want 1", deps.userID)
				}

				return nil
			},
		)

		if err := handler(context.Background(), tg.Entities{}, &tg.UpdateBotInlineQuery{}); err != nil {
			t.Fatalf("wrapAsync() error = %v", err)
		}

		if !called.Load() {
			t.Fatal("wrapAsync() did not dispatch synchronously")
		}
	})

	t.Run("dispatches asynchronously without scheduler", func(t *testing.T) {
		t.Parallel()

		dispatched := make(chan struct{})
		handler := wrapAsync(
			false,
			nil,
			func(_ tg.Entities, _ *tg.UpdateBotInlineQuery) (UpdateData, bool) {
				return UpdateData{userID: 2}, true
			},
			func(_ context.Context, deps UpdateData) yaerrors.Error {
				if deps.userID != 2 {
					t.Fatalf("dispatch userID = %d, want 2", deps.userID)
				}

				close(dispatched)

				return nil
			},
		)

		if err := handler(context.Background(), tg.Entities{}, &tg.UpdateBotInlineQuery{}); err != nil {
			t.Fatalf("wrapAsync() error = %v", err)
		}

		waitForSignal(t, dispatched, "async dispatch without scheduler")
	})
}

func TestBindWithSyncDisablesSequentialScheduler(t *testing.T) {
	t.Parallel()

	dispatcher := &Dispatcher{
		Features:        FeatureSequentialUpdates,
		updateScheduler: newAsyncUpdateScheduler(),
	}
	tgDispatcher := tg.NewUpdateDispatcher()

	dispatcher.Bind(&tgDispatcher, true)

	if dispatcher.updateScheduler != nil {
		t.Fatal("Bind(sync=true) unexpectedly kept the sequential scheduler")
	}
}

func TestDispatchUsesFallbackLocalizerForUnknownLanguage(t *testing.T) {
	t.Parallel()

	root := NewRouterGroup()
	root.OnMessage(func(
		_ context.Context,
		handlerData *HandlerData,
		_ *tg.UpdateNewMessage,
	) yaerrors.Error {
		if handlerData.Localizer == nil {
			t.Fatal("handlerData.Localizer = nil, want fallback localizer")
		}

		return nil
	})

	dispatcher := &Dispatcher{
		Log:        yalogger.NewBaseLogger(nil).NewLogger(),
		Localizer:  yalocales.NewLocalizer("en", true),
		MainRouter: root,
	}

	err := dispatcher.dispatch(
		context.Background(),
		UpdateData{
			userID: 1,
			chatID: 99,
			ent:    testEntities(),
			update: &tg.UpdateNewMessage{},
		},
	)
	if err != nil {
		t.Fatalf("dispatch() error = %v", err)
	}
}

func TestInitYaTgBotRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	_, err := InitYaTgBot(context.Background(), Options{
		BotToken: "invalid-token",
	})
	if err == nil {
		t.Fatal("InitYaTgBot() error = nil, want invalid bot token error")
	}
}
