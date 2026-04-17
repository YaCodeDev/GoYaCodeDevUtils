package yatgbot

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/gotd/td/tg"
)

func TestDispatchKeepsRootRouterAcrossSubrouters(t *testing.T) {
	t.Parallel()

	root := NewRouterGroup()
	interactive := NewRouterGroup()
	payment := NewRouterGroup()
	root.IncludeRouter(interactive, payment)

	handled := 0
	interactive.OnMessage(func(
		_ context.Context,
		_ *HandlerData,
		_ *tg.UpdateNewMessage,
	) yaerrors.Error {
		handled++

		return nil
	})
	payment.OnPrecheckoutQuery(func(
		_ context.Context,
		_ *HandlerData,
		_ *tg.UpdateBotPrecheckoutQuery,
	) yaerrors.Error {
		return nil
	})

	dispatcher := &Dispatcher{
		Log:        yalogger.NewBaseLogger(nil).NewLogger(),
		MainRouter: root,
	}

	deps := UpdateData{
		userID: 1,
		chatID: 1,
		ent:    tg.Entities{},
		update: &tg.UpdateNewMessage{},
	}

	for range 2 {
		if err := dispatcher.dispatch(context.Background(), deps); err != nil {
			t.Fatalf("dispatch() error = %v", err)
		}
	}

	if handled != 2 {
		t.Fatalf("handled update count = %d, want 2", handled)
	}

	if dispatcher.MainRouter != root {
		t.Fatal("dispatcher.MainRouter changed after dispatch")
	}
}
