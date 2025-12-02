package yatgmessageencoding

type urlLikeState struct {
	startPosition  multiSize
	middlePosition multiSize
	URL            string
}

type urlLikeStates struct {
	linkOrEmoji urlLikeState
	emoji       urlLikeState
}

type multiSize struct {
	utf8      uint32
	utf16LECU uint32
}

func (m multiSize) Add(other multiSize) multiSize {
	return multiSize{
		utf8:      m.utf8 + other.utf8,
		utf16LECU: m.utf16LECU + other.utf16LECU,
	}
}

func (m multiSize) Sub(other multiSize) multiSize {
	return multiSize{
		utf8:      m.utf8 - other.utf8,
		utf16LECU: m.utf16LECU - other.utf16LECU,
	}
}
