// Package yaimagesupport centralizes image decoder registration used by
// YaCodeDev services.
//
// Call Init from service startup before decoding images. Call InitCGO instead
// when the binary is built with CGO_ENABLED=1 and -tags yaimagesupport_native.
// Most decoders are loaded by imported packages through their own init
// functions; these Init functions are explicit startup hooks for this package's
// custom registrations and native-support checks.
package yaimagesupport
