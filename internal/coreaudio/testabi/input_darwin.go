//go:build darwin && (amd64 || arm64)

// Package testabi supplies a foreign AudioUnitRender stand-in for protocol
// tests. Production code never imports it.
package testabi

func fakeInputRender()

var InputRenderABI uintptr
