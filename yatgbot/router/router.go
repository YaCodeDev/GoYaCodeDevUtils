package router

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

// MessageHandler is a function that processes incoming messages.
// CallbackHandler is a function that processes incoming callback queries.
type (
	MessageHandler  func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateNewMessage) yaerrors.Error
	CallbackHandler func(ctx context.Context, handlerData *HandlerData, cb *tg.UpdateBotCallbackQuery) yaerrors.Error
)

// route represents a single route in the router.
type route struct {
	filters    []Filter
	msgHandler MessageHandler
	cbHandler  CallbackHandler
}

// Router is the main struct that holds routes, sub-routers, and middlewares.
type Router struct {
	Dependencies

	parent      *Router
	name        string
	base        []Filter
	sub         []*Router
	routes      []*route
	middlewares []HandlerMiddleware
}

// Dependencies holds the external dependencies required by the Router.
type Dependencies struct {
	FSMStore          yafsm.FSM
	Log               yalogger.Logger
	MessageDispatcher *messagequeue.Dispatcher
	Localizer         yalocales.Localizer
	Client            *tg.Client
	Sender            *message.Sender
}

// New creates a new Router instance with the given name and dependencies.
// If deps is nil, it initializes with default zero values.
//
// Example of usage:
//
// r := router.New("main", YourDependencies)
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

// IncludeRouter includes sub-routers into the current router.
// It sets the parent and inherits dependencies if they are not set.
//
// Example of usage:
//
// subRouter := New("sub", nil)
//
// mainRouter := New("main", YourDependencies)
//
// mainRouter.IncludeRouter(subRouter)
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

// OnMessage registers a message handler with optional filters.
//
// Example of usage:
//
// router.OnMessage(YourMessageHandler, YourFilter1, YourFilter2)
func (r *Router) OnMessage(h MessageHandler, filters ...Filter) {
	r.routes = append(r.routes, &route{
		msgHandler: h,
		filters:    filters,
	})
}

// OnCallback registers a callback handler with optional filters.
//
// Example of usage:
//
// router.OnCallback(YourCallbackHandler, YourFilter1, YourFilter2)
func (r *Router) OnCallback(h CallbackHandler, filters ...Filter) {
	r.routes = append(r.routes, &route{
		cbHandler: h,
		filters:   filters,
	})
}
