package yatgmessageencoding

func getUTF16LECUSize(s string) uint32 {
	if len(s) == 0 {
		return 0
	}

	var size uint32

	for _, runeVal := range s {
		if runeVal <= maxOneUTF16LECUSizedRune {
			size++
		} else {
			size += 2
		}
	}

	return size
}

func getUTF8Size(s string) uint32 {
	return uint32(len(s)) //nolint:gosec
}

func getMultiSize(s string) multiSize {
	return multiSize{
		utf8:      getUTF8Size(s),
		utf16LECU: getUTF16LECUSize(s),
	}
}
