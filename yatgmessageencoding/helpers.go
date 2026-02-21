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
	return uint32( //nolint:gosec // Overflow is not a concern here as input string is expected to be reasonably sized
		len(s),
	)
}

func getMultiSize(s string) multiSize {
	return multiSize{
		utf8:      getUTF8Size(s),
		utf16LECU: getUTF16LECUSize(s),
	}
}
