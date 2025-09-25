package router

import (
	"context"
	"net/http"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/gotd/td/tg"
)

// DispatcherDependencies holds the dependencies required for dispatching an update.
type DispatcherDependencies struct {
	userID    int64
	chatID    int64
	ent       tg.Entities
	update    tg.UpdateClass
	inputPeer tg.InputPeerClass
}

// dispatch processes the update by checking filters and executing the appropriate handler.
// It also supports nested routers by dispatching to sub-routers if no local route matches.
func (r *Router) dispatch(ctx context.Context, deps DispatcherDependencies) yaerrors.Error {
	userFSMStorage := yafsm.NewUserFSMStorage(
		r.FSMStore,
		strconv.FormatInt(deps.chatID, 10),
	)

	r.Log.Debugf("Processing update: %+v with entities: %+v", deps.update, deps.ent)

	for _, rt := range r.routes {
		ok, err := r.checkFilters(
			ctx,
			FilterDependencies{
				update:  deps.update,
				storage: *userFSMStorage,
				userID:  deps.userID,
			},
			rt.filters)
		if err != nil {
			return yaerrors.FromErrorWithLog(http.StatusInternalServerError, err, "failed to apply filters", r.Log)
		}

		if !ok {
			r.Log.Debugf("Filters not passed for %T", deps.update)

			continue
		}
		var localizer yalocales.Localizer

		if user, ok := deps.ent.Users[deps.userID]; ok && user.LangCode != "" {
			localizer, err = r.Localizer.DeriveNewDefaultLang(user.LangCode)

			if err != nil {
				if err != yalocales.ErrInvalidLanguage {
					return yaerrors.FromErrorWithLog(
						http.StatusInternalServerError,
						err,
						"failed to derive localizer",
						r.Log,
					)
				}

				localizer = r.Localizer
			}
			r.Log.Debugf("Using user %d language: %s", deps.userID, user.LangCode)
		}

		hdata := &HandlerData{
			Entities:     deps.ent,
			Sender:       r.Sender,
			Update:       deps.update,
			UserID:       deps.userID,
			Peer:         deps.inputPeer,
			StateStorage: userFSMStorage,
			Log:          r.Log,
			Dispatcher:   r.MessageDispatcher,
			Localizer:    localizer,
			Client:       r.Client,
		}

		switch u := deps.update.(type) {
		case *tg.UpdateNewMessage:
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

// checkFilters checks the filters of the current router and its parents recursively.
func (r *Router) checkFilters(
	ctx context.Context,
	deps FilterDependencies,
	local []Filter,
) (bool, yaerrors.Error) {
	if r.parent != nil {
		ok, err := r.parent.checkFilters(ctx, deps, nil)
		if err != nil || !ok {
			return ok, err.Wrap("parent filter check failed")
		}
	}

	for _, f := range r.base {
		ok, err := f(ctx, deps)
		if err != nil || !ok {
			return ok, err.Wrap("base filter check failed")
		}
	}

	for _, f := range local {
		ok, err := f(ctx, deps)
		if err != nil || !ok {
			return ok, err.Wrap("local filter check failed")
		}
	}

	return true, nil
}

// makeInputPeer converts a tg.PeerClass to a tg.InputPeerClass using the provided entities.
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

// getChatID extracts the chat ID from a tg.PeerClass using the provided entities.
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

// getUserID extracts the user ID from a tg.PeerClass or from the FromID field if available.
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
