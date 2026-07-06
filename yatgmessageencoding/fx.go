package yatgmessageencoding

import "go.uber.org/fx"

// MessageEncodingModuleName is the fx module name for the yatgmessageencoding providers.
const MessageEncodingModuleName = "yatgmessageencoding"

// MessageEncodingModule provides a MessageEncoding backed by markdownEncoding.
//
// Example usage:
//
//	fx.New(yatgmessageencoding.MessageEncodingModule)
var MessageEncodingModule = fx.Module(
	MessageEncodingModuleName,
	fx.Provide(NewMarkdownEncoding),
)
