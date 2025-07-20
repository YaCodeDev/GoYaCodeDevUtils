package config

import "regexp"

const (
	DefaultTagName = "default"
	DotEnvFile     = ".env"
	DotEnvKVParts  = 2
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
	intStringMap
	intIntMap
	intUintMap
	intFloatMap
	intBoolMap
	uintStringMap
	uintIntMap
	uintUintMap
	uintFloatMap
	uintBoolMap
	floatStringMap
	floatIntMap
	floatUintMap
	floatFloatMap
	floatBoolMap
	boolStringMap
	boolIntMap
	boolUintMap
	boolFloatMap
	boolBoolMap
	invalidMap
)
