package router

import (
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/fsm"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgbot/messagequeue"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

type HandlerData struct {
	Entities   tg.Entities
	Sender     *message.Sender
	Client     *tg.Client
	Update     any
	UserID     int64
	Peer       tg.InputPeerClass
	State      *fsm.UserFSMStorage
	Log        yalogger.Logger
	Dispatcher *messagequeue.Dispatcher
	T          func(string) string // localizer
}
