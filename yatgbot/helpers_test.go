package yatgbot

import (
	"context"
	"regexp"
	"slices"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gotd/td/tg"
)

type testState struct {
	yafsm.BaseState[testState]
}

func TestFeatureFlagsHas(t *testing.T) {
	t.Parallel()

	flags := FeatureSequentialUpdates

	if !flags.Has(FeatureSequentialUpdates) {
		t.Fatal("expected feature flag to be enabled")
	}

	if flags.Has(FeatureFlags(1 << 1)) {
		t.Fatal("unexpected feature flag reported as enabled")
	}
}

func TestUpdateDataSequencingKeys(t *testing.T) {
	t.Parallel()

	t.Run("includes user and chat keys", func(t *testing.T) {
		t.Parallel()

		deps := UpdateData{
			userID: 17,
			chatID: 42,
		}

		got := deps.sequencingKeys()
		want := []string{"user:17", "chat:42"}

		if !slices.Equal(got, want) {
			t.Fatalf("sequencingKeys() = %v, want %v", got, want)
		}
	})

	t.Run("omits zero keys", func(t *testing.T) {
		t.Parallel()

		deps := UpdateData{}

		if got := deps.sequencingKeys(); len(got) != 0 {
			t.Fatalf("sequencingKeys() = %v, want empty slice", got)
		}
	})
}

func TestPeerHelpers(t *testing.T) {
	t.Parallel()

	ent := testEntities()

	t.Run("makeInputPeer", func(t *testing.T) {
		t.Parallel()

		inputPeer, ok := makeInputPeer(&tg.PeerUser{UserID: 1}, ent)
		if !ok {
			t.Fatal("makeInputPeer() did not resolve user peer")
		}

		userPeer, ok := inputPeer.(*tg.InputPeerUser)
		if !ok {
			t.Fatalf("makeInputPeer() type = %T, want *tg.InputPeerUser", inputPeer)
		}

		if userPeer.UserID != 1 || userPeer.AccessHash != 111 {
			t.Fatalf("makeInputPeer() user = %+v, want userID=1 accessHash=111", userPeer)
		}

		inputPeer, ok = makeInputPeer(&tg.PeerChat{ChatID: 99}, ent)
		if !ok {
			t.Fatal("makeInputPeer() did not resolve chat peer")
		}

		chatPeer, ok := inputPeer.(*tg.InputPeerChat)
		if !ok || chatPeer.ChatID != 99 {
			t.Fatalf("makeInputPeer() chat = %#v, want chatID=99", inputPeer)
		}

		inputPeer, ok = makeInputPeer(&tg.PeerChannel{ChannelID: 7}, ent)
		if !ok {
			t.Fatal("makeInputPeer() did not resolve channel peer")
		}

		channelPeer, ok := inputPeer.(*tg.InputPeerChannel)
		if !ok {
			t.Fatalf("makeInputPeer() type = %T, want *tg.InputPeerChannel", inputPeer)
		}

		if channelPeer.ChannelID != 7 || channelPeer.AccessHash != 777 {
			t.Fatalf("makeInputPeer() channel = %+v, want channelID=7 accessHash=777", channelPeer)
		}

		if _, ok := makeInputPeer(&tg.PeerUser{UserID: 404}, ent); ok {
			t.Fatal("makeInputPeer() unexpectedly resolved missing user peer")
		}

		if _, ok := makeInputPeer(&tg.PeerChannel{ChannelID: 404}, ent); ok {
			t.Fatal("makeInputPeer() unexpectedly resolved missing channel peer")
		}

		if _, ok := makeInputPeer(nil, ent); ok {
			t.Fatal("makeInputPeer() unexpectedly resolved nil peer")
		}
	})

	t.Run("getChatID", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			peer tg.PeerClass
			want int64
			ok   bool
		}{
			{name: "user", peer: &tg.PeerUser{UserID: 1}, want: 1, ok: true},
			{name: "chat", peer: &tg.PeerChat{ChatID: 99}, want: 99, ok: true},
			{name: "channel", peer: &tg.PeerChannel{ChannelID: 7}, want: 7, ok: true},
			{name: "missing channel", peer: &tg.PeerChannel{ChannelID: 404}, ok: false},
			{name: "nil", peer: nil, ok: false},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, ok := getChatID(tc.peer, ent)
				if ok != tc.ok || got != tc.want {
					t.Fatalf("getChatID() = (%d, %v), want (%d, %v)", got, ok, tc.want, tc.ok)
				}
			})
		}
	})

	t.Run("getUserID", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			peer   tg.PeerClass
			fromID tg.PeerClass
			want   int64
			ok     bool
		}{
			{name: "user peer", peer: &tg.PeerUser{UserID: 1}, want: 1, ok: true},
			{name: "chat from user", peer: &tg.PeerChat{ChatID: 99}, fromID: &tg.PeerUser{UserID: 2}, want: 2, ok: true},
			{name: "chat without user", peer: &tg.PeerChat{ChatID: 99}, ok: false},
			{name: "channel", peer: &tg.PeerChannel{ChannelID: 7}, fromID: &tg.PeerUser{UserID: 2}, ok: false},
			{name: "nil", peer: nil, ok: false},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, ok := getUserID(tc.peer, tc.fromID)
				if ok != tc.ok || got != tc.want {
					t.Fatalf("getUserID() = (%d, %v), want (%d, %v)", got, ok, tc.want, tc.ok)
				}
			})
		}
	})
}

func TestCheckFilters(t *testing.T) {
	t.Parallel()

	dispatcher := &Dispatcher{}
	root := NewRouterGroup()
	child := NewRouterGroup()
	root.IncludeRouter(child)

	t.Run("runs parent then child then local filters", func(t *testing.T) {
		t.Parallel()

		order := make([]string, 0, 3)
		root.base = []Filter{func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
			order = append(order, "root")

			return true, nil
		}}
		child.base = []Filter{func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
			order = append(order, "child")

			return true, nil
		}}

		ok, err := dispatcher.checkFilters(
			context.Background(),
			child,
			FilterDependencies{},
			[]Filter{func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				order = append(order, "local")

				return true, nil
			}},
		)
		if err != nil {
			t.Fatalf("checkFilters() error = %v", err)
		}

		if !ok {
			t.Fatal("checkFilters() unexpectedly returned false")
		}

		want := []string{"root", "child", "local"}
		if !slices.Equal(order, want) {
			t.Fatalf("checkFilters() order = %v, want %v", order, want)
		}
	})

	t.Run("returns false when a filter rejects", func(t *testing.T) {
		t.Parallel()

		root.base = []Filter{func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
			return false, nil
		}}
		child.base = nil

		ok, err := dispatcher.checkFilters(context.Background(), child, FilterDependencies{}, nil)
		if err != nil {
			t.Fatalf("checkFilters() error = %v", err)
		}

		if ok {
			t.Fatal("checkFilters() unexpectedly returned true")
		}
	})

	t.Run("wraps filter errors", func(t *testing.T) {
		t.Parallel()

		root.base = []Filter{func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
			return false, yaerrors.FromString(500, "boom")
		}}
		child.base = nil

		ok, err := dispatcher.checkFilters(context.Background(), child, FilterDependencies{}, nil)
		if err == nil {
			t.Fatal("checkFilters() error = nil, want wrapped error")
		}

		if ok {
			t.Fatal("checkFilters() unexpectedly returned true")
		}
	})
}

func TestDispatchRouterHandlesNilRouterAndFilterErrors(t *testing.T) {
	t.Parallel()

	dispatcher := &Dispatcher{
		Log:        yalogger.NewBaseLogger(nil).NewLogger(),
		Localizer:  yalocales.NewLocalizer("en", true),
		MainRouter: NewRouterGroup(),
	}

	if err := dispatcher.dispatchRouter(context.Background(), nil, UpdateData{}); err != nil {
		t.Fatalf("dispatchRouter(nil) error = %v", err)
	}

	dispatcher.MainRouter.OnMessage(
		func(_ context.Context, _ *HandlerData, _ *tg.UpdateNewMessage) yaerrors.Error {
			return nil
		},
		func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
			return false, yaerrors.FromString(500, "boom")
		},
	)

	err := dispatcher.dispatchRouter(
		context.Background(),
		dispatcher.MainRouter,
		UpdateData{
			update: &tg.UpdateNewMessage{},
		},
	)
	if err == nil {
		t.Fatal("dispatchRouter() error = nil, want wrapped filter error")
	}
}

func TestFilters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	msgUpdate := &tg.UpdateNewMessage{
		Message: &tg.Message{
			Message: "hello world",
		},
	}
	serviceUpdate := &tg.UpdateNewMessage{
		Message: &tg.MessageService{
			Action: &tg.MessageActionChatCreate{Title: "room"},
		},
	}
	callbackUpdate := &tg.UpdateBotCallbackQuery{
		Data: []byte("prefix_value"),
	}

	t.Run("StateIs", func(t *testing.T) {
		t.Parallel()

		cache := yacache.NewCache(yacache.NewMemoryContainer())
		fsm := yafsm.NewDefaultFSMStorage(cache, yafsm.EmptyState{})
		storage := yafsm.NewUserFSMStorage(fsm, "42")
		filter := StateIs(testState{}.StateName())

		if err := storage.SetState(ctx, testState{}); err != nil {
			t.Fatalf("SetState() error = %v", err)
		}

		ok, err := filter(ctx, FilterDependencies{storage: *storage})
		if err != nil {
			t.Fatalf("StateIs() error = %v", err)
		}

		if !ok {
			t.Fatal("StateIs() unexpectedly returned false")
		}

		if err := cache.Set(ctx, "42", "{", 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		ok, err = filter(ctx, FilterDependencies{storage: *storage})
		if err == nil {
			t.Fatal("StateIs() error = nil, want wrapped storage error")
		}

		if ok {
			t.Fatal("StateIs() unexpectedly returned true")
		}
	})

	t.Run("TextEq", func(t *testing.T) {
		t.Parallel()

		ok, err := TextEq("hello world")(ctx, FilterDependencies{update: msgUpdate})
		if err != nil || !ok {
			t.Fatalf("TextEq() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = TextEq("other")(ctx, FilterDependencies{update: msgUpdate})
		if err != nil || ok {
			t.Fatalf("TextEq() = (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("TextRegex", func(t *testing.T) {
		t.Parallel()

		ok, err := TextRegex(regexp.MustCompile(`^hello`))(ctx, FilterDependencies{update: msgUpdate})
		if err != nil || !ok {
			t.Fatalf("TextRegex() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = TextRegex(regexp.MustCompile(`^other`))(ctx, FilterDependencies{update: msgUpdate})
		if err != nil || ok {
			t.Fatalf("TextRegex() = (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("Callback filters", func(t *testing.T) {
		t.Parallel()

		ok, err := CallbackEq("prefix_value")(ctx, FilterDependencies{update: callbackUpdate})
		if err != nil || !ok {
			t.Fatalf("CallbackEq() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = CallbackPrefix("prefix_")(ctx, FilterDependencies{update: callbackUpdate})
		if err != nil || !ok {
			t.Fatalf("CallbackPrefix() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = CallbackEq("other")(ctx, FilterDependencies{update: callbackUpdate})
		if err != nil || ok {
			t.Fatalf("CallbackEq() = (%v, %v), want (false, nil)", ok, err)
		}

		ok, err = CallbackPrefix("other")(ctx, FilterDependencies{update: callbackUpdate})
		if err != nil || ok {
			t.Fatalf("CallbackPrefix() = (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("Message service filters", func(t *testing.T) {
		t.Parallel()

		ok, err := MessageServiceActionFilter[*tg.MessageActionChatCreate]()(ctx, FilterDependencies{update: serviceUpdate})
		if err != nil || !ok {
			t.Fatalf("MessageServiceActionFilter() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = MessageServiceFilter()(ctx, FilterDependencies{update: serviceUpdate})
		if err != nil || !ok {
			t.Fatalf("MessageServiceFilter() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = MessageServiceActionFilter[*tg.MessageActionChatJoinedByLink]()(ctx, FilterDependencies{update: serviceUpdate})
		if err != nil || ok {
			t.Fatalf("MessageServiceActionFilter() = (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("OneOfFilter", func(t *testing.T) {
		t.Parallel()

		ok, err := OneOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return false, nil
			},
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return true, nil
			},
		)(ctx, FilterDependencies{})
		if err != nil || !ok {
			t.Fatalf("OneOfFilter() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = OneOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return false, nil
			},
		)(ctx, FilterDependencies{})
		if err != nil || ok {
			t.Fatalf("OneOfFilter() = (%v, %v), want (false, nil)", ok, err)
		}

		ok, err = OneOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return false, yaerrors.FromString(500, "boom")
			},
		)(ctx, FilterDependencies{})
		if err == nil || ok {
			t.Fatalf("OneOfFilter() = (%v, %v), want wrapped error", ok, err)
		}
	})

	t.Run("AllOfFilter", func(t *testing.T) {
		t.Parallel()

		ok, err := AllOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return true, nil
			},
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return true, nil
			},
		)(ctx, FilterDependencies{})
		if err != nil || !ok {
			t.Fatalf("AllOfFilter() = (%v, %v), want (true, nil)", ok, err)
		}

		ok, err = AllOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return false, nil
			},
		)(ctx, FilterDependencies{})
		if err != nil || ok {
			t.Fatalf("AllOfFilter() = (%v, %v), want (false, nil)", ok, err)
		}

		ok, err = AllOfFilter(
			func(_ context.Context, _ FilterDependencies) (bool, yaerrors.Error) {
				return false, yaerrors.FromString(500, "boom")
			},
		)(ctx, FilterDependencies{})
		if err == nil || ok {
			t.Fatalf("AllOfFilter() = (%v, %v), want wrapped error", ok, err)
		}
	})
}

func TestRouterRegistrationAndMiddlewares(t *testing.T) {
	t.Parallel()

	router := NewRouterGroup()
	filter := TextEq("hello")

	router.OnCallback(func(_ context.Context, _ *HandlerData, _ *tg.UpdateBotCallbackQuery) yaerrors.Error { return nil }, filter)
	router.OnMessage(func(_ context.Context, _ *HandlerData, _ *tg.UpdateNewMessage) yaerrors.Error { return nil }, filter)
	router.OnEditMessage(func(_ context.Context, _ *HandlerData, _ *tg.UpdateEditMessage) yaerrors.Error { return nil }, filter)
	router.OnDeleteMessage(func(_ context.Context, _ *HandlerData, _ *tg.UpdateDeleteMessages) yaerrors.Error { return nil }, filter)
	router.OnNewChannelMessage(func(_ context.Context, _ *HandlerData, _ *tg.UpdateNewChannelMessage) yaerrors.Error { return nil }, filter)
	router.OnEditChannelMessage(func(_ context.Context, _ *HandlerData, _ *tg.UpdateEditChannelMessage) yaerrors.Error { return nil }, filter)
	router.OnDeleteChannelMessages(func(_ context.Context, _ *HandlerData, _ *tg.UpdateDeleteChannelMessages) yaerrors.Error { return nil }, filter)
	router.OnMessageReactions(func(_ context.Context, _ *HandlerData, _ *tg.UpdateMessageReactions) yaerrors.Error { return nil }, filter)
	router.OnChannelParticipant(func(_ context.Context, _ *HandlerData, _ *tg.UpdateChannelParticipant) yaerrors.Error { return nil }, filter)
	router.OnPrecheckoutQuery(func(_ context.Context, _ *HandlerData, _ *tg.UpdateBotPrecheckoutQuery) yaerrors.Error { return nil }, filter)
	router.OnInlineQuery(func(_ context.Context, _ *HandlerData, _ *tg.UpdateBotInlineQuery) yaerrors.Error { return nil }, filter)

	if got, want := len(router.routes), 11; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}

	root := NewRouterGroup()
	child := NewRouterGroup()
	root.IncludeRouter(child)

	root.AddMiddleware(func(
		ctx context.Context,
		handlerData *HandlerData,
		upd tg.UpdateClass,
		next HandlerNext,
	) yaerrors.Error {
		return next(ctx, handlerData, upd)
	})
	child.AddMiddleware(func(
		ctx context.Context,
		handlerData *HandlerData,
		upd tg.UpdateClass,
		next HandlerNext,
	) yaerrors.Error {
		return next(ctx, handlerData, upd)
	})

	if got, want := len(child.collectMiddlewares()), 2; got != want {
		t.Fatalf("collectMiddlewares() len = %d, want %d", got, want)
	}

	order := make([]string, 0, 2)
	handler := chainMiddleware(
		func(_ context.Context, _ *HandlerData, _ tg.UpdateClass) yaerrors.Error {
			order = append(order, "handler")

			return nil
		},
		func(
			ctx context.Context,
			handlerData *HandlerData,
			upd tg.UpdateClass,
			next HandlerNext,
		) yaerrors.Error {
			order = append(order, "mw")

			return next(ctx, handlerData, upd)
		},
	)

	if err := handler(context.Background(), &HandlerData{}, &tg.UpdateNewMessage{}); err != nil {
		t.Fatalf("chainMiddleware() error = %v", err)
	}

	if want := []string{"mw", "handler"}; !slices.Equal(order, want) {
		t.Fatalf("chainMiddleware() order = %v, want %v", order, want)
	}
}

func TestExtractMessageHelpers(t *testing.T) {
	t.Parallel()

	message := &tg.Message{Message: "hello"}
	service := &tg.MessageService{Action: &tg.MessageActionChatCreate{Title: "room"}}

	if got, ok := ExtractMessageFromUpdate(&tg.UpdateNewMessage{Message: message}); !ok || got != message {
		t.Fatalf("ExtractMessageFromUpdate() = (%v, %v), want (%v, true)", got, ok, message)
	}

	if got, ok := ExtractMessageFromUpdate(&tg.UpdateNewChannelMessage{Message: message}); !ok || got != message {
		t.Fatalf("ExtractMessageFromUpdate(channel) = (%v, %v), want (%v, true)", got, ok, message)
	}

	if got, ok := ExtractMessageFromUpdate(&tg.UpdateNewMessage{Message: service}); ok || got != nil {
		t.Fatalf("ExtractMessageFromUpdate(service) = (%v, %v), want (nil, false)", got, ok)
	}

	if got, ok := ExtractMessageServiceFromUpdate(&tg.UpdateNewMessage{Message: service}); !ok || got != service {
		t.Fatalf("ExtractMessageServiceFromUpdate() = (%v, %v), want (%v, true)", got, ok, service)
	}

	if got, ok := ExtractMessageServiceFromUpdate(&tg.UpdateNewChannelMessage{Message: service}); !ok || got != service {
		t.Fatalf("ExtractMessageServiceFromUpdate(channel) = (%v, %v), want (%v, true)", got, ok, service)
	}

	if got, ok := ExtractMessageServiceFromUpdate(&tg.UpdateNewMessage{Message: message}); ok || got != nil {
		t.Fatalf("ExtractMessageServiceFromUpdate(message) = (%v, %v), want (nil, false)", got, ok)
	}
}

func testEntities() tg.Entities {
	return tg.Entities{
		Users: map[int64]*tg.User{
			1: {
				ID:         1,
				AccessHash: 111,
				LangCode:   "ua",
			},
			2: {
				ID:         2,
				AccessHash: 222,
			},
			99: {
				ID:         99,
				AccessHash: 999,
			},
		},
		Chats: map[int64]*tg.Chat{
			99: {
				ID: 99,
			},
		},
		Channels: map[int64]*tg.Channel{
			7: {
				ID:         7,
				AccessHash: 777,
			},
		},
	}
}
