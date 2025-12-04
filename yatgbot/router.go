package yatgbot

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/gotd/td/tg"
)

// HandlerData holds the dependencies and context for a handler execution.
type HandlerData struct {
	Entities     tg.Entities
	Client       *yatgclient.Client
	Update       tg.UpdateClass
	UserID       int64
	Peer         tg.InputPeerClass
	StateStorage *yafsm.EntityFSMStorage
	Log          yalogger.Logger
	Dispatcher   *messagequeue.Dispatcher
	Localizer    yalocales.Localizer
	JobResults   []messagequeue.JobResult
}

type (
	// CallbackHandler is a function that processes incoming callback queries.
	CallbackHandler func(ctx context.Context, handlerData *HandlerData, cb *tg.UpdateBotCallbackQuery) yaerrors.Error

	// NewMessageHandler is a function that processes incoming messages.
	NewMessageHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateNewMessage) yaerrors.Error

	EditMessageHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateEditMessage) yaerrors.Error

	DeleteMessageHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateDeleteMessages) yaerrors.Error

	NewChannelMessageHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateNewChannelMessage) yaerrors.Error

	EditChannelMessageHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateEditChannelMessage) yaerrors.Error

	DeleteChannelMessagesHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateDeleteChannelMessages) yaerrors.Error

	MessageReactionsHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateMessageReactions) yaerrors.Error

	ChannelParticipantHandler func(ctx context.Context, handlerData *HandlerData, msg *tg.UpdateChannelParticipant) yaerrors.Error

	// PrecheckoutQueryHandler is a function that processes incoming pre-checkout queries.
	PrecheckoutQueryHandler func(ctx context.Context, handlerData *HandlerData, query *tg.UpdateBotPrecheckoutQuery) yaerrors.Error

	InlineQueryHandler func(ctx context.Context, handlerData *HandlerData, query *tg.UpdateBotInlineQuery) yaerrors.Error
)

// RouterGroup is the main struct that holds routes, sub-routers, and middlewares.
type RouterGroup struct {
	parent      *RouterGroup
	base        []Filter
	sub         []*RouterGroup
	routes      []route
	middlewares []HandlerMiddleware
}

// Dependencies holds the external dependencies required by the Router.
// route represents a single route in the router.
type route struct {
	filters []Filter
	handler HandlerNext
}

// NewRouterGroup creates a new Router instance with the given name.
//
// Example usage:
//
// r := router.NewRouterGroup("main", YourDependencies)
func NewRouterGroup() *RouterGroup {
	return &RouterGroup{}
}

// IncludeRouter includes sub-routers into the current router.
// It sets the parent and inherits dependencies if they are not set.
//
// Example usage:
//
// subRouter := New("sub", nil)
//
// mainRouter := New("main", YourDependencies)
//
// mainRouter.IncludeRouter(subRouter)
func (r *RouterGroup) IncludeRouter(subs ...*RouterGroup) {
	for _, s := range subs {
		s.parent = r

		r.sub = append(r.sub, s)
	}
}

// OnCallback registers a callback handler with optional filters.
//
// Example usage:
//
// router.OnCallback(YourCallbackHandler, YourFilter1, YourFilter2)
func (r *RouterGroup) OnCallback(h CallbackHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

// OnMessage registers a message handler with optional filters.
//
// Example usage:
//
// router.OnMessage(YourMessageHandler, YourFilter1, YourFilter2)
func (r *RouterGroup) OnMessage(h NewMessageHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnEditMessage(h EditMessageHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnDeleteMessage(h DeleteMessageHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnNewChannelMessage(h NewChannelMessageHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnEditChannelMessage(h EditChannelMessageHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnDeleteChannelMessages(h DeleteChannelMessagesHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnMessageReactions(h MessageReactionsHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnChannelParticipant(h ChannelParticipantHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnPrecheckoutQuery(h PrecheckoutQueryHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}

func (r *RouterGroup) OnInlineQuery(h InlineQueryHandler, filters ...Filter) {
	r.routes = append(r.routes, route{
		handler: wrapHandler(h),
		filters: filters,
	})
}
