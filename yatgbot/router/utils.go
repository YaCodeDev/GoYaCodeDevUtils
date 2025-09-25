package router

import "github.com/gotd/td/tg"

func extractMessageFromUpdate(upd tg.UpdateClass) (*tg.Message, bool) {
	switch u := upd.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			return msg, true
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			return msg, true
		}
	}

	return nil, false
}
