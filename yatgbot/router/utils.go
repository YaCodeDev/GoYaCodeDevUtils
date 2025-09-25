package router

import "github.com/gotd/td/tg"

// extractMessageFromUpdate tries to extract a *tg.Message from the given update.
// It returns the message and true if successful, otherwise nil and false.
//
// Example of usage:
//
// msg, ok := extractMessageFromUpdate(update)
//
//	if ok {
//		 // process msg
//	}
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
