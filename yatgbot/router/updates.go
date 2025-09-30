package router

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
// Example of usage:
//
// // dispatcher := tg.NewUpdateDispatcher(yourClient)
//
// r := router.New("main", YourDependencies)
// r.Bind(dispatcher)
func (r *Router) Bind(d *tg.UpdateDispatcher) {
	d.OnNewMessage(r.wrapMessage)
	d.OnBotCallbackQuery(r.wrapCallback)
}

// wrapMessage wraps the message handler to match the expected signature for the update dispatcher.
func (r *Router) wrapMessage(ctx context.Context, ent tg.Entities, upd *tg.UpdateNewMessage) error {
	msg, ok := upd.Message.(*tg.Message)
	if !ok {
		return nil
	}

	uid, _ := getUserID(msg.PeerID, msg.FromID)

	chatID, _ := getChatID(msg.PeerID, ent)

	peer, _ := makeInputPeer(msg.PeerID, ent)

	return r.dispatch(ctx, DispatcherDependencies{
		userID:    uid,
		chatID:    chatID,
		ent:       ent,
		update:    upd,
		inputPeer: peer,
	})
}

// wrapCallback wraps the callback query handler to match the expected signature for the update dispatcher.
func (r *Router) wrapCallback(
	ctx context.Context,
	ent tg.Entities,
	q *tg.UpdateBotCallbackQuery,
) error {
	chatID, _ := getChatID(q.Peer, ent)

	peer, _ := makeInputPeer(q.Peer, ent)

	return r.dispatch(ctx, DispatcherDependencies{
		userID:    q.UserID,
		chatID:    chatID,
		ent:       ent,
		update:    q,
		inputPeer: peer,
	})
}
