package yatgbot

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

type shortUpdateNormalizer struct {
	next telegram.UpdateHandler
}

func newShortUpdateNormalizer(next telegram.UpdateHandler) telegram.UpdateHandler {
	return shortUpdateNormalizer{next: next}
}

func (h shortUpdateNormalizer) Handle(ctx context.Context, updates tg.UpdatesClass) error {
	if h.next == nil {
		return nil
	}

	if err := h.next.Handle(ctx, normalizeShortUpdates(updates)); err != nil {
		return errors.Wrap(err, "handle normalized short update")
	}

	return nil
}

func normalizeShortUpdates(updates tg.UpdatesClass) tg.UpdatesClass {
	switch update := updates.(type) {
	case *tg.UpdateShortMessage:
		if update.Out {
			return updates
		}

		return &tg.Updates{
			Updates: []tg.UpdateClass{
				&tg.UpdateNewMessage{
					Message:  shortMessageToMessage(update),
					Pts:      update.Pts,
					PtsCount: update.PtsCount,
				},
			},
			Date: update.Date,
		}
	case *tg.UpdateShortChatMessage:
		if update.Out {
			return updates
		}

		return &tg.Updates{
			Updates: []tg.UpdateClass{
				&tg.UpdateNewMessage{
					Message:  shortChatMessageToMessage(update),
					Pts:      update.Pts,
					PtsCount: update.PtsCount,
				},
			},
			Date: update.Date,
		}
	default:
		return updates
	}
}

func shortMessageToMessage(update *tg.UpdateShortMessage) *tg.Message {
	return &tg.Message{
		Out:         update.Out,
		Mentioned:   update.Mentioned,
		MediaUnread: update.MediaUnread,
		Silent:      update.Silent,
		ID:          update.ID,
		FromID:      &tg.PeerUser{UserID: update.UserID},
		PeerID:      &tg.PeerUser{UserID: update.UserID},
		FwdFrom:     update.FwdFrom,
		ViaBotID:    update.ViaBotID,
		ReplyTo:     update.ReplyTo,
		Date:        update.Date,
		Message:     update.Message,
		Entities:    update.Entities,
		TTLPeriod:   update.TTLPeriod,
	}
}

func shortChatMessageToMessage(update *tg.UpdateShortChatMessage) *tg.Message {
	return &tg.Message{
		Out:         update.Out,
		Mentioned:   update.Mentioned,
		MediaUnread: update.MediaUnread,
		Silent:      update.Silent,
		ID:          update.ID,
		FromID:      &tg.PeerUser{UserID: update.FromID},
		PeerID:      &tg.PeerChat{ChatID: update.ChatID},
		FwdFrom:     update.FwdFrom,
		ViaBotID:    update.ViaBotID,
		ReplyTo:     update.ReplyTo,
		Date:        update.Date,
		Message:     update.Message,
		Entities:    update.Entities,
		TTLPeriod:   update.TTLPeriod,
	}
}
