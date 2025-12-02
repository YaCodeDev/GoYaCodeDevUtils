package yatgmessageencoding

import (
	"github.com/gotd/td/tg"
)

// MessageEncoding defines interface for Telegram (gotd) message encoding intended to use with YaTgClient or YaTgBot
type MessageEncoding interface {
	// Parse parses the input text and returns the encoded text along with the corresponding message entities.
	// According to Telegram specifications, entity offsets are in UTF-16LE code units.
	//
	// Example usage:
	//	md := yatgmessageencoding.NewMarkdownEncoding()
	//	inputText := "This is **bold** text"
	//	encodedText, entities := md.Parse(inputText)
	Parse(text string) (string, []tg.MessageEntityClass)

	// Unparse takes the text and its associated message entities, and reconstructs the original formatted text.
	// According to Telegram specifications, entity offsets are in UTF-16LE code units.
	//
	// Example usage:
	//	md := yatgmessageencoding.NewMarkdownEncoding()
	//	text := "This is bold text"
	//	entities := []tg.MessageEntityClass{
	//		&tg.MessageEntityBold{Offset: 8, Length: 4},
	//	}
	//	unparsedText := md.Unparse(text, entities)
	Unparse(text string, entities []tg.MessageEntityClass) string
}
