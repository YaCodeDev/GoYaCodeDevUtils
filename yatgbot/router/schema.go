package router

import (
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

// HandlerData holds the dependencies and context for a handler execution.
type HandlerData struct {
	Entities     tg.Entities
	Sender       *message.Sender
	Client       *tg.Client
	Update       tg.UpdateClass
	UserID       int64
	Peer         tg.InputPeerClass
	StateStorage *yafsm.EntityFSMStorage
	Log          yalogger.Logger
	Dispatcher   *messagequeue.Dispatcher
	T            func(string) string // localizer
}
