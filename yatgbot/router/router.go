package router

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/localizer"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

type (
	MessageHandler  func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateNewMessage) yaerrors.Error
	CallbackHandler func(ctx context.Context, handlerData *HandlerData, cb *tg.UpdateBotCallbackQuery) yaerrors.Error
)

type route struct {
	filters    []Filter
	msgHandler MessageHandler
	cbHandler  CallbackHandler
}

type Router struct {
	Dependencies

	parent      *Router
	name        string
	base        []Filter
	sub         []*Router
	routes      []*route
	middlewares []HandlerMiddleware
}

type Dependencies struct {
	FSMStore          yafsm.FSM
	Log               yalogger.Logger
	MessageDispatcher *messagequeue.Dispatcher
	Localizer         *localizer.Localizer
	Client            *tg.Client
	Sender            *message.Sender
}

func New(name string, deps *Dependencies) *Router {
	if deps == nil {
		deps = &Dependencies{}
	}

	r := &Router{
		name:         name,
		Dependencies: *deps,
	}

	return r
}

func (r *Router) Use(f ...Filter) { r.base = append(r.base, f...) }

func (r *Router) IncludeRouter(subs ...*Router) {
	for _, s := range subs {
		s.parent = r

		if s.Sender == nil {
			s.Sender = r.Sender
		}

		if s.FSMStore == nil {
			s.FSMStore = r.FSMStore
		}

		if s.Log == nil {
			s.Log = r.Log
		}

		if s.MessageDispatcher == nil {
			s.MessageDispatcher = r.MessageDispatcher
		}

		if s.Localizer == nil {
			s.Localizer = r.Localizer
		}

		if s.Client == nil {
			s.Client = r.Client
		}

		r.sub = append(r.sub, s)
	}
}

func (r *Router) OnMessage(h MessageHandler, filters ...Filter) {
	r.routes = append(r.routes, &route{
		msgHandler: h,
		filters:    filters,
	})
}

func (r *Router) OnCallback(h CallbackHandler, filters ...Filter) {
	r.routes = append(r.routes, &route{
		cbHandler: h,
		filters:   filters,
	})
}
