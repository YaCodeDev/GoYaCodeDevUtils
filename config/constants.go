package config

import "regexp"

const (
	MapPartsCount = 2
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// This type is used to define the types of supported maps.
type mapType uint8

const (
	stringStringMap mapType = iota
	stringIntMap
	stringUintMap
	stringFloatMap
	stringBoolMap
	stringByteSliceMap
	intStringMap
	intIntMap
	intUintMap
	intFloatMap
	intBoolMap
	intByteSliceMap
	uintStringMap
	uintIntMap
	uintUintMap
	uintFloatMap
	uintBoolMap
	uintByteSliceMap
	floatStringMap
	floatIntMap
	floatUintMap
	floatFloatMap
	floatBoolMap
	floatByteSliceMap
	boolStringMap
	boolIntMap
	boolUintMap
	boolFloatMap
	boolBoolMap
	boolByteSliceMap
	invalidMap
)
