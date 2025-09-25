package router

import (
	"context"

	"github.com/gotd/td/tg"
)

func (r *Router) Bind(d *tg.UpdateDispatcher) {
	d.OnNewMessage(r.wrapMessage)
	d.OnBotCallbackQuery(r.wrapCallback)
}

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
