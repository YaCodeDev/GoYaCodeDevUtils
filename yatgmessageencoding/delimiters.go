package yatgmessageencoding

import (
	"strconv"

	"github.com/gotd/td/tg"
)

const lineBreak = '\n'

type delimiter string

const (
	preDelim        delimiter = "```"
	boldDelim       delimiter = "**"
	italicDelim     delimiter = "__"
	underlineDelim  delimiter = "++"
	strikeDelim     delimiter = "~~"
	spoilerDelim    delimiter = "||"
	quoteDelim      delimiter = "&&"
	codeDelim       delimiter = "`"
	linkStartDelim  delimiter = "["
	linkMiddleDelim delimiter = "]("
	linkEndDelim    delimiter = ")"
	escapeDelim     delimiter = "\\"
)

var allStandardDelimiters = map[delimiter]struct{}{
	preDelim:       {},
	boldDelim:      {},
	italicDelim:    {},
	underlineDelim: {},
	strikeDelim:    {},
	spoilerDelim:   {},
	quoteDelim:     {},
	codeDelim:      {},
}

var allURLLikeDelimiters = map[delimiter]struct{}{
	linkStartDelim:  {},
	linkMiddleDelim: {},
	linkEndDelim:    {},
}

var allDelimiters = func() map[delimiter]struct{} {
	m := make(map[delimiter]struct{}, len(allStandardDelimiters)+len(allURLLikeDelimiters))

	for _, k := range []map[delimiter]struct{}{allStandardDelimiters, allURLLikeDelimiters} {
		for k := range k {
			m[k] = struct{}{}
		}
	}

	return m
}()

var largestDelimiterSize = func() multiSize {
	var maxSize multiSize

	for d := range allDelimiters {
		size := d.Size()
		if size.utf8 > maxSize.utf8 {
			maxSize.utf8 = size.utf8
		}

		if size.utf16LECU > maxSize.utf16LECU {
			maxSize.utf16LECU = size.utf16LECU
		}
	}

	return maxSize
}()

var delimitersSizes = func() map[delimiter]multiSize {
	m := make(map[delimiter]multiSize, len(allDelimiters)+1)

	for d := range allDelimiters {
		m[d] = getMultiSize(d.String())
	}

	m[escapeDelim] = getMultiSize(escapeDelim.String())

	return m
}()

var charsToEscape = func() map[rune]struct{} {
	m := make(map[rune]struct{}, len(delimitersSizes))

	for d := range delimitersSizes {
		for _, r := range d.String() {
			m[r] = struct{}{}
		}
	}

	return m
}()

func (d delimiter) String() string {
	return string(d)
}

func (d delimiter) Size() multiSize {
	return delimitersSizes[d]
}

func (d delimiter) CreateEntity(
	offsetUTF16LECU, lengthUTF16LECU uint32,
	customParams ...string,
) tg.MessageEntityClass {
	offset := int(offsetUTF16LECU)
	length := int(lengthUTF16LECU)

	switch d {
	case boldDelim:
		return &tg.MessageEntityBold{Offset: offset, Length: length}
	case italicDelim:
		return &tg.MessageEntityItalic{Offset: offset, Length: length}
	case underlineDelim:
		return &tg.MessageEntityUnderline{Offset: offset, Length: length}
	case strikeDelim:
		return &tg.MessageEntityStrike{Offset: offset, Length: length}
	case spoilerDelim:
		return &tg.MessageEntitySpoiler{Offset: offset, Length: length}
	case quoteDelim:
		return &tg.MessageEntityBlockquote{Offset: offset, Length: length}
	case codeDelim:
		return &tg.MessageEntityCode{Offset: offset, Length: length}
	case preDelim:
		var language string

		if len(customParams) > 0 {
			language = customParams[0]
		}

		return &tg.MessageEntityPre{Offset: offset, Length: length, Language: language}
	case linkStartDelim, linkMiddleDelim, linkEndDelim:
		var url string

		if len(customParams) == 0 {
			return nil
		}

		url = customParams[0]

		if id, err := strconv.ParseInt(url, 10, 64); err == nil {
			return &tg.MessageEntityCustomEmoji{Offset: offset, Length: length, DocumentID: id}
		}

		return &tg.MessageEntityTextURL{Offset: offset, Length: length, URL: customParams[0]}
	case escapeDelim:
		return nil
	default:
		return nil
	}
}

func getDelimiterForEntity(entity tg.MessageEntityClass) delimiter {
	switch entity.(type) {
	case *tg.MessageEntityBold:
		return boldDelim
	case *tg.MessageEntityItalic:
		return italicDelim
	case *tg.MessageEntityUnderline:
		return underlineDelim
	case *tg.MessageEntityStrike:
		return strikeDelim
	case *tg.MessageEntitySpoiler:
		return spoilerDelim
	case *tg.MessageEntityBlockquote:
		return quoteDelim
	case *tg.MessageEntityCode:
		return codeDelim
	case *tg.MessageEntityPre:
		return preDelim
	case *tg.MessageEntityTextURL, *tg.MessageEntityCustomEmoji:
		return linkStartDelim
	default:
		return ""
	}
}
