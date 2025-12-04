package yatgbot

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/gotd/td/tg"
)

type Dispatcher struct {
	FSMStore          yafsm.FSM
	Log               yalogger.Logger
	BotUser           *tg.User
	MessageDispatcher *messagequeue.Dispatcher
	Localizer         yalocales.Localizer
	Client            *yatgclient.Client
	MainRouter        *RouterGroup
}

// UpdateData holds the dependencies required for dispatching an update.
type UpdateData struct {
	userID    int64
	chatID    int64
	ent       tg.Entities
	update    tg.UpdateClass
	inputPeer tg.InputPeerClass
}

// dispatch processes the update by checking filters and executing the appropriate handler.
// It also supports nested routers by dispatching to sub-routers if no local route matches.
func (r *Dispatcher) dispatch(ctx context.Context, deps UpdateData) yaerrors.Error {
	userFSMStorage := yafsm.NewUserFSMStorage(
		r.FSMStore,
		strconv.FormatInt(deps.chatID, 10),
	)

	r.Log.Debugf("Processing update: %+v with entities: %+v", deps.update, deps.ent)

	for _, rt := range r.MainRouter.routes {
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
				if !errors.Is(err, yalocales.ErrInvalidLanguage) {
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
			Update:       deps.update,
			UserID:       deps.userID,
			Peer:         deps.inputPeer,
			StateStorage: userFSMStorage,
			Log:          r.Log,
			Dispatcher:   r.MessageDispatcher,
			Localizer:    localizer,
			Client:       r.Client,
		}

		err = chainMiddleware(rt.handler, r.MainRouter.collectMiddlewares()...)(ctx, hdata, deps.update)
		if err != nil {
			if errors.Is(err, ErrRouteMismatch) {
				continue
			}
			return err.Wrap("handler execution failed")
		}

		return nil
	}

	for _, sub := range r.MainRouter.sub {
		r.MainRouter = sub

		err := r.dispatch(ctx, deps)
		if err != nil {
			return err.Wrap("sub-router dispatch failed")
		}
	}

	return nil
}

// checkFilters checks the filters of the current router and its parents recursively.
func (r *Dispatcher) checkFilters(
	ctx context.Context,
	deps FilterDependencies,
	local []Filter,
) (bool, yaerrors.Error) {
	// 1) Build the chain from current group up to root.
	var chain []*RouterGroup
	for g := r.MainRouter; g != nil; g = g.parent {
		chain = append(chain, g)
	}

	// 2) Reverse so we run filters from root -> current.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	// 3) Run base filters in that order.
	for _, grp := range chain {
		for _, f := range grp.base {
			ok, err := f(ctx, deps)
			if err != nil {
				return false, err.Wrap("base filter check failed")
			}

			if !ok {
				return false, nil
			}
		}
	}

	// 4) Run local (route) filters last.
	for _, f := range local {
		ok, err := f(ctx, deps)
		if err != nil {
			return false, err.Wrap("local filter check failed")
		}

		if !ok {
			return false, nil
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
