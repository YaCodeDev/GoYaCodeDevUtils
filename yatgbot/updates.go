package yatgbot

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/tg"
)

// Bind binds the router to the given update dispatcher.
// It sets up updates handling for bot.
// It should be called once during the bot setup.
// After calling this method, the router will start receiving updates
// and dispatching them to the appropriate handlers based on the defined routes and filters.
//
// Example usage:
//
// router := yatgbot.NewRouterGroup()
//
// dispatcher := tg.NewUpdateDispatcher(yourClient)
//
// router.Bind(dispatcher)
func (r *Dispatcher) Bind(tgDispatcher *tg.UpdateDispatcher, sync bool) {
	if !r.Features.Has(FeatureSequentialUpdates) || sync {
		r.updateScheduler = nil
	} else if r.updateScheduler == nil {
		r.updateScheduler = newAsyncUpdateScheduler()
	}

	tgDispatcher.OnNewMessage(
		wrapAsync(sync, r.updateScheduler, r.buildNewMessageUpdateData, r.dispatch),
	)
	tgDispatcher.OnBotCallbackQuery(
		wrapAsync(sync, r.updateScheduler, r.buildBotCallbackQueryUpdateData, r.dispatch),
	)
	tgDispatcher.OnDeleteMessages(
		wrapAsync(sync, r.updateScheduler, r.buildDeleteMessagesUpdateData, r.dispatch),
	)
	tgDispatcher.OnEditMessage(
		wrapAsync(sync, r.updateScheduler, r.buildEditMessageUpdateData, r.dispatch),
	)
	tgDispatcher.OnNewChannelMessage(
		wrapAsync(sync, r.updateScheduler, r.buildNewChannelMessageUpdateData, r.dispatch),
	)
	tgDispatcher.OnEditChannelMessage(
		wrapAsync(sync, r.updateScheduler, r.buildEditChannelMessageUpdateData, r.dispatch),
	)
	tgDispatcher.OnChannelParticipant(
		wrapAsync(sync, r.updateScheduler, r.buildChannelParticipantUpdateData, r.dispatch),
	)
	tgDispatcher.OnDeleteChannelMessages(
		wrapAsync(sync, r.updateScheduler, r.buildDeleteChannelMessagesUpdateData, r.dispatch),
	)
	tgDispatcher.OnBotMessageReactions(
		wrapAsync(sync, r.updateScheduler, r.buildBotMessageReactionsUpdateData, r.dispatch),
	)
	tgDispatcher.OnBotPrecheckoutQuery(
		wrapAsync(sync, r.updateScheduler, r.buildBotPrecheckoutQueryUpdateData, r.dispatch),
	)
	tgDispatcher.OnBotInlineQuery(
		wrapAsync(sync, r.updateScheduler, r.buildBotInlineQueryUpdateData, r.dispatch),
	)
}

func (r *Dispatcher) buildNewMessageUpdateData(
	ent tg.Entities,
	upd *tg.UpdateNewMessage,
) (UpdateData, bool) {
	switch msg := upd.Message.(type) {
	case *tg.Message:
		if msg.FromID != nil {
			if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
				if fromUser.UserID == r.BotUser.ID {
					return UpdateData{}, false
				}
			}
		}

		return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
	case *tg.MessageService:
		return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
	default:
		return UpdateData{}, false
	}
}

func (r *Dispatcher) buildBotCallbackQueryUpdateData(
	ent tg.Entities,
	q *tg.UpdateBotCallbackQuery,
) (UpdateData, bool) {
	chatID, ok := getChatID(q.Peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	inputPeer, ok := makeInputPeer(q.Peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	return UpdateData{
		userID:    q.UserID,
		chatID:    chatID,
		ent:       ent,
		update:    q,
		inputPeer: inputPeer,
	}, true
}

func (r *Dispatcher) buildNewChannelMessageUpdateData(
	ent tg.Entities,
	upd *tg.UpdateNewChannelMessage,
) (UpdateData, bool) {
	switch msg := upd.Message.(type) {
	case *tg.Message:
		return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
	case *tg.MessageService:
		return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
	default:
		return UpdateData{}, false
	}
}

func (r *Dispatcher) buildBotPrecheckoutQueryUpdateData(
	ent tg.Entities,
	upd *tg.UpdateBotPrecheckoutQuery,
) (UpdateData, bool) {
	return buildUserScopedUpdateData(ent, upd, upd.UserID)
}

func (r *Dispatcher) buildEditMessageUpdateData(
	ent tg.Entities,
	upd *tg.UpdateEditMessage,
) (UpdateData, bool) {
	msg, ok := upd.Message.(*tg.Message)
	if !ok {
		return UpdateData{}, false
	}

	return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
}

func (r *Dispatcher) buildBotInlineQueryUpdateData(
	ent tg.Entities,
	upd *tg.UpdateBotInlineQuery,
) (UpdateData, bool) {
	return buildUserScopedUpdateData(ent, upd, upd.UserID)
}

func (r *Dispatcher) buildEditChannelMessageUpdateData(
	ent tg.Entities,
	upd *tg.UpdateEditChannelMessage,
) (UpdateData, bool) {
	msg, ok := upd.Message.(*tg.Message)
	if !ok {
		return UpdateData{}, false
	}

	return buildPeerUpdateData(ent, upd, msg.PeerID, msg.FromID)
}

func (r *Dispatcher) buildDeleteMessagesUpdateData(
	ent tg.Entities,
	upd *tg.UpdateDeleteMessages,
) (UpdateData, bool) {
	return UpdateData{
		userID:    0,
		chatID:    0,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	}, true
}

func (r *Dispatcher) buildDeleteChannelMessagesUpdateData(
	ent tg.Entities,
	upd *tg.UpdateDeleteChannelMessages,
) (UpdateData, bool) {
	return UpdateData{
		userID:    0,
		chatID:    upd.ChannelID,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	}, true
}

func (r *Dispatcher) buildChannelParticipantUpdateData(
	ent tg.Entities,
	upd *tg.UpdateChannelParticipant,
) (UpdateData, bool) {
	return UpdateData{
		userID:    upd.UserID,
		chatID:    upd.ChannelID,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	}, true
}

func (r *Dispatcher) buildBotMessageReactionsUpdateData(
	ent tg.Entities,
	upd *tg.UpdateBotMessageReactions,
) (UpdateData, bool) {
	chatID, ok := getChatID(upd.Peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	inputPeer, ok := makeInputPeer(upd.Peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	return UpdateData{
		userID:    0,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: inputPeer,
	}, true
}

func buildUserScopedUpdateData(
	ent tg.Entities,
	upd tg.UpdateClass,
	userID int64,
) (UpdateData, bool) {
	user, ok := ent.Users[userID]
	if !ok {
		return UpdateData{}, false
	}

	var (
		chatID    int64
		inputPeer tg.InputPeerClass
	)

	if len(ent.Chats) > 0 {
		for _, chat := range ent.Chats {
			chatID = chat.ID

			break
		}

		inputPeer = &tg.InputPeerChat{
			ChatID: chatID,
		}
	} else {
		chatID = userID
		inputPeer = &tg.InputPeerUser{
			UserID:     userID,
			AccessHash: user.AccessHash,
		}
	}

	return UpdateData{
		userID:    userID,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: inputPeer,
	}, true
}

func buildPeerUpdateData(
	ent tg.Entities,
	upd tg.UpdateClass,
	peer tg.PeerClass,
	fromID tg.PeerClass,
) (UpdateData, bool) {
	chatID, ok := getChatID(peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	inputPeer, ok := makeInputPeer(peer, ent)
	if !ok {
		return UpdateData{}, false
	}

	userID, _ := getUserID(peer, fromID)

	return UpdateData{
		userID:    userID,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: inputPeer,
	}, true
}

// wrapAsync wraps the handler to run asynchronously if sync is false.
func wrapAsync[T tg.UpdateClass](
	sync bool,
	scheduler *asyncUpdateScheduler,
	build func(tg.Entities, T) (UpdateData, bool),
	dispatch func(context.Context, *UpdateData) yaerrors.Error,
) func(context.Context, tg.Entities, T) error {
	return func(ctx context.Context, e tg.Entities, upd T) error {
		deps, ok := build(e, upd)
		if !ok {
			return nil
		}

		if sync {
			return dispatch(ctx, &deps)
		}

		if scheduler == nil {
			go func() {
				_ = dispatch( //nolint:errcheck,lll // It isn't really possible to do anything about the error here, as it is run asynchronously and the handler is responsible for its own error handling and logging.
					ctx,
					&deps,
				)
			}()

			return nil
		}

		scheduler.Enqueue(deps.sequencingKeys(), func() {
			_ = dispatch( //nolint:errcheck,lll // It isn't really possible to do anything about the error here, as it is run asynchronously and the handler is responsible for its own error handling and logging.
				ctx,
				&deps,
			)
		})

		return nil
	}
}
