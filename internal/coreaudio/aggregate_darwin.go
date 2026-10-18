//go:build darwin && (amd64 || arm64)

package coreaudio

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"time"
)

const privateAggregatePrefix = "m31labs.dev/tymbal/duplex/"

// The AUHAL serves one hardware clock. Separate endpoints use a process-private
// aggregate, with input as the clock source and output drift compensation.
// Core Audio owns that conversion; the stream never drops or repeats input
// frames to pretend that independent devices have the same clock.
func newDuplexDevice(outputUID, inputUID string, outputs, inputs int) (uint32, error) {
	var description bytes.Buffer
	writeString := func(s string) {
		description.WriteString("<string>")
		_ = xml.EscapeText(&description, []byte(s))
		description.WriteString("</string>")
	}
	description.WriteString(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>uid</key>`)
	writeString(privateAggregatePrefix + rand.Text())
	description.WriteString(`<key>name</key><string>Tymbal duplex clock</string><key>private</key><integer>1</integer><key>master</key>`)
	writeString(inputUID)
	description.WriteString(`<key>subdevices</key><array><dict><key>uid</key>`)
	writeString(outputUID)
	description.WriteString(`<key>drift</key><integer>1</integer></dict><dict><key>uid</key>`)
	writeString(inputUID)
	description.WriteString(`</dict></array></dict></plist>`)
	// Parsing a property list gives the dictionary standard CF ownership and
	// equality callbacks, without retaining Go pointers in Core Foundation.
	plist := cfPropertyList(description.Bytes())
	if plist == 0 {
		return 0, fmt.Errorf("coreaudio: could not build the duplex device description")
	}
	defer cfRelease(plist)
	var device uint32
	if err := check("AudioHardwareCreateAggregateDevice", createAggregateDevice(plist, &device)); err != nil {
		return 0, classify(err)
	}
	// HAL subdevices can activate asynchronously. This is setup, never an IO
	// callback; wait briefly for the endpoint geometry rather than failing a
	// healthy first open while the audio server is still assembling the device.
	deadline := time.Now().Add(time.Second)
	for {
		if channelCount(device, scopeOutput) >= outputs && channelCount(device, scopeInput) >= inputs {
			return device, nil
		}
		if time.Now().After(deadline) {
			_ = destroyAggregateDevice(device)
			return 0, fmt.Errorf("coreaudio: duplex device channels did not become ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
