package router

import (
	"context"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/fsm"
	"github.com/gotd/td/tg"
)

type DispatcherDependecies struct {
	userID    int64
	chatID    int64
	ent       tg.Entities
	update    any
	inputPeer tg.InputPeerClass
}

func (r *Router) dispatch(ctx context.Context, deps DispatcherDependecies) yaerrors.Error {
	userFSMStorage := fsm.NewUserFSMStorage(
		r.FSMStore,
		strconv.FormatInt(deps.chatID, 10),
	)

	r.Log.Debugf("Coming update: %+v\nEntities: %+v", deps.update, deps.ent)

	for _, rt := range r.routes {
		ok, err := r.checkFilters(
			ctx,
			FilterDependecies{
				update:  deps.update,
				storage: *userFSMStorage,
				userID:  deps.userID,
			},
			rt.filters)
		if err != nil {
			return yaerrors.FromErrorWithLog(0, err, "failed to apply filters", r.Log)
		}

		if !ok {
			r.Log.Debugf("Filters not passed for %T", deps.update)

			continue
		}

		var lang func(string) string

		if user, ok := deps.ent.Users[deps.userID]; ok && user.LangCode != "" {
			lang = r.Localizer.Lang(user.LangCode)
			r.Log.Debugf("Using user %d language: %s", deps.userID, user.LangCode)
		}

		hdata := &HandlerData{
			Entities:   deps.ent,
			Sender:     r.Sender,
			Update:     deps.update,
			UserID:     deps.userID,
			Peer:       deps.inputPeer,
			State:      userFSMStorage,
			Log:        r.Log,
			Dispatcher: r.MessageDispatcher,
			T:          lang,
			Client:     r.Client,
		}

		switch u := deps.update.(type) {
		case *tg.Message:
			return chainMiddleware(wrapHandler(rt.msgHandler), r.collectMiddlewares()...)(ctx, hdata, u)
		case *tg.UpdateBotCallbackQuery:
			return chainMiddleware(wrapHandler(rt.cbHandler), r.collectMiddlewares()...)(ctx, hdata, u)
		}
	}

	for _, sub := range r.sub {
		err := sub.dispatch(ctx, deps)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Router) checkFilters(ctx context.Context, deps FilterDependecies, local []Filter) (bool, yaerrors.Error) {
	if r.parent != nil {
		ok, err := r.parent.checkFilters(ctx, deps, nil)
		if err != nil || !ok {
			return ok, err
		}
	}

	for _, f := range r.base {
		ok, err := f(ctx, deps)
		if err != nil || !ok {
			return ok, err
		}
	}

	for _, f := range local {
		ok, err := f(ctx, deps)
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

func makeInputPeer(p tg.PeerClass, ents tg.Entities) (tg.InputPeerClass, bool) {
	switch v := p.(type) {
	case *tg.PeerUser:
		u, ok := ents.Users[v.UserID]
		if !ok {
			return nil, false
		}

		return &tg.InputPeerUser{
			UserID:     v.UserID,
			AccessHash: u.AccessHash,
		}, true

	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: v.ChatID}, true

	case *tg.PeerChannel:
		c, ok := ents.Channels[v.ChannelID]
		if !ok {
			return nil, false
		}

		return &tg.InputPeerChannel{
			ChannelID:  v.ChannelID,
			AccessHash: c.AccessHash,
		}, true
	}

	return nil, false
}

func getChatID(peer tg.PeerClass, ents tg.Entities) (int64, bool) {
	switch v := peer.(type) {
	case *tg.PeerUser:
		return v.UserID, true
	case *tg.PeerChat:
		return v.ChatID, true
	case *tg.PeerChannel:
		c, ok := ents.Channels[v.ChannelID]
		if !ok {
			return 0, false
		}

		return c.ID, true
	default:
		return 0, false
	}
}

func getUserID(peer tg.PeerClass, fromID tg.PeerClass) (int64, bool) {
	switch v := peer.(type) {
	case *tg.PeerUser:
		return v.UserID, true

	case *tg.PeerChat:
		if fromUser, ok := fromID.(*tg.PeerUser); ok {
			return fromUser.UserID, true
		}

		return 0, false

	default:
		return 0, false
	}
}
