package yatgmessageencoding

const (
	maxUint32                 = ^uint32(0)
	entitiesSliceInitialCap   = 64
	entitiesSliceGrowthFactor = 2
	maxOneUTF16LECUSizedRune  = 0xFFFF
)

var maxMultiSize = multiSize{utf8: maxUint32, utf16LECU: maxUint32}
