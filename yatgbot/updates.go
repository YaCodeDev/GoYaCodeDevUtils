package yatgbot

import (
	"context"

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
func (r *Dispatcher) Bind(tgDispatcher *tg.UpdateDispatcher) {
	tgDispatcher.OnNewMessage(r.handleNewMessage)
	tgDispatcher.OnBotCallbackQuery(r.handleBotCallbackQuery)
	tgDispatcher.OnDeleteMessages(r.handleDeleteMessages)
	tgDispatcher.OnEditMessage(r.handleEditMessage)
	tgDispatcher.OnNewChannelMessage(r.handleNewChannelMessage)
	tgDispatcher.OnEditChannelMessage(r.handleEditChannelMessage)
	tgDispatcher.OnChannelParticipant(r.handleChannelParticipant)
	tgDispatcher.OnDeleteChannelMessages(r.handleDeleteChannelMessages)
	tgDispatcher.OnBotMessageReactions(r.handleBotMessageReactions)
	tgDispatcher.OnBotPrecheckoutQuery(r.handleBotPrecheckoutQuery)
	tgDispatcher.OnBotInlineQuery(r.handleBotInlineQuery)
}

// handleNewMessage wraps the new message handler to match the expected signature for the update dispatcher.
func (r *Dispatcher) handleNewMessage(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateNewMessage,
) error {
	var (
		uid    int64
		chatID int64
		peer   tg.InputPeerClass
	)

	switch msg := upd.Message.(type) {
	case *tg.Message:
		if msg.FromID != nil {
			if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
				if fromUser.UserID == r.BotUser.ID {
					return nil
				}
			}
		}

		uid, _ = getUserID(msg.PeerID, msg.FromID)

		chatID, _ = getChatID(msg.PeerID, ent)

		peer, _ = makeInputPeer(msg.PeerID, ent)

	case *tg.MessageService:
		uid, _ = getUserID(msg.PeerID, msg.FromID)

		chatID, _ = getChatID(msg.PeerID, ent)

		peer, _ = makeInputPeer(msg.PeerID, ent)
	default:
		return nil
	}

	return r.dispatch(ctx, UpdateData{
		userID:    uid,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}

// handleBotCallbackQuery wraps the callback query handler to match the expected signature for the update dispatcher.
func (r *Dispatcher) handleBotCallbackQuery(
	ctx context.Context,
	ent tg.Entities,
	q *tg.UpdateBotCallbackQuery,
) error {
	chatID, _ := getChatID(q.Peer, ent)

	peer, _ := makeInputPeer(q.Peer, ent)

	return r.dispatch(ctx, UpdateData{
		userID:    q.UserID,
		chatID:    chatID,
		ent:       ent,
		update:    q,
		inputPeer: peer,
	})
}

// handleNewChannelMessage wraps the new channel message handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleNewChannelMessage(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateNewChannelMessage,
) error {
	var (
		uid    int64
		chatID int64
		peer   tg.InputPeerClass
	)

	switch msg := upd.Message.(type) {
	case *tg.Message:
		uid, _ = getUserID(msg.PeerID, msg.FromID)

		chatID, _ = getChatID(msg.PeerID, ent)

		peer, _ = makeInputPeer(msg.PeerID, ent)

	case *tg.MessageService:
		uid, _ = getUserID(msg.PeerID, msg.FromID)

		chatID, _ = getChatID(msg.PeerID, ent)

		peer, _ = makeInputPeer(msg.PeerID, ent)
	default:
		return nil
	}

	return r.dispatch(ctx, UpdateData{
		userID:    uid,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}

// handleBotPrecheckoutQuery wraps the pre-checkout query handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleBotPrecheckoutQuery(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateBotPrecheckoutQuery,
) error {
	user, ok := ent.Users[upd.UserID]
	if !ok {
		return nil
	}

	var (
		chatID    int64
		inputPeer tg.InputPeerClass
	)

	if len(ent.Chats) > 0 {
		chatID = ent.Chats[0].ID
		inputPeer = &tg.InputPeerChat{
			ChatID: chatID,
		}
	} else {
		chatID = upd.UserID
		inputPeer = &tg.InputPeerUser{
			UserID:     upd.UserID,
			AccessHash: user.AccessHash,
		}
	}

	return r.dispatch(ctx, UpdateData{
		userID:    upd.UserID,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: inputPeer,
	})
}

// handleEditMessage wraps the edit message handler to match the expected signature for the update dispatcher.
func (r *Dispatcher) handleEditMessage(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateEditMessage,
) error {
	r.Log.Infof("EditMessage: %+v", upd)

	msg, ok := upd.Message.(*tg.Message)
	if !ok {
		return nil
	}

	if msg.FromID != nil {
		if fromUser, okPeer := msg.FromID.(*tg.PeerUser); okPeer {
			if fromUser.UserID == r.BotUser.ID {
				return nil
			}
		}
	}

	invoice, ok := msg.Media.(*tg.MessageMediaInvoice)

	if ok {
		r.Log.Infof("Invoice received: %+v", invoice)
	}

	uid, _ := getUserID(msg.PeerID, msg.FromID)

	chatID, _ := getChatID(msg.PeerID, ent)

	peer, _ := makeInputPeer(msg.PeerID, ent)

	return r.dispatch(ctx, UpdateData{
		userID:    uid,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}

// handleBotInlineQuery wraps the inline query handler to match the expected signature for the update dispatcher.
func (r *Dispatcher) handleBotInlineQuery(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateBotInlineQuery,
) error {
	user, ok := ent.Users[upd.UserID]
	if !ok {
		return nil
	}

	var (
		chatID    int64
		inputPeer tg.InputPeerClass
	)

	if len(ent.Chats) > 0 {
		chatID = ent.Chats[0].ID
		inputPeer = &tg.InputPeerChat{
			ChatID: chatID,
		}
	} else {
		chatID = upd.UserID
		inputPeer = &tg.InputPeerUser{
			UserID:     upd.UserID,
			AccessHash: user.AccessHash,
		}
	}

	return r.dispatch(ctx, UpdateData{
		userID:    upd.UserID,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: inputPeer,
	})
}

// handleEditChannelMessage wraps the edit channel message handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleEditChannelMessage(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateEditChannelMessage,
) error {
	msg, ok := upd.Message.(*tg.Message)
	if !ok {
		return nil
	}

	uid, _ := getUserID(msg.PeerID, msg.FromID)

	chatID, _ := getChatID(msg.PeerID, ent)

	peer, _ := makeInputPeer(msg.PeerID, ent)

	return r.dispatch(ctx, UpdateData{
		userID:    uid,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}

// handleDeleteMessages wraps the delete messages handler to match the expected signature for the update dispatcher.
func (r *Dispatcher) handleDeleteMessages(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateDeleteMessages,
) error {
	return r.dispatch(ctx, UpdateData{
		userID:    0,
		chatID:    0,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	})
}

// handleDeleteChannelMessages wraps the delete channel messages handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleDeleteChannelMessages(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateDeleteChannelMessages,
) error {
	return r.dispatch(ctx, UpdateData{
		userID:    0,
		chatID:    upd.ChannelID,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	})
}

// handleChannelParticipant wraps the channel participant handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleChannelParticipant(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateChannelParticipant,
) error {
	return r.dispatch(ctx, UpdateData{
		userID:    upd.UserID,
		chatID:    upd.ChannelID,
		ent:       ent,
		update:    upd,
		inputPeer: nil,
	})
}

// handleBotMessageReactions wraps the message reactions handler to match
// the expected signature for the update dispatcher.
func (r *Dispatcher) handleBotMessageReactions(
	ctx context.Context,
	ent tg.Entities,
	upd *tg.UpdateBotMessageReactions,
) error {
	chatID, _ := getChatID(upd.Peer, ent)

	peer, _ := makeInputPeer(upd.Peer, ent)

	return r.dispatch(ctx, UpdateData{
		userID:    0,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}
