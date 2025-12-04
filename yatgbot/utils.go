package yatgbot

import "github.com/gotd/td/tg"

// ExtractMessageFromUpdate tries to extract a *tg.Message from the given update.
// It returns the message and true if successful, otherwise nil and false.
//
// Example usage:
//
// msg, ok := ExtractMessageFromUpdate(update)
//
//	if ok {
//		 // process msg
//	}
func ExtractMessageFromUpdate(upd tg.UpdateClass) (*tg.Message, bool) {
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

// ExtractMessageServiceFromUpdate tries to extract a *tg.MessageService from the given update.
// It returns the message service and true if successful, otherwise nil and false.
//
// Example usage:
//
// msgService, ok := ExtractMessageServiceFromUpdate(update)
//
//	if ok {
//		 // process msgService
//	}
func ExtractMessageServiceFromUpdate(upd tg.UpdateClass) (*tg.MessageService, bool) {
	switch u := upd.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := u.Message.(*tg.MessageService); ok {
			return msg, true
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := u.Message.(*tg.MessageService); ok {
			return msg, true
		}
	}

	return nil, false
}
