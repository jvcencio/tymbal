package tymbal

import (
	"context"

	"m31labs.dev/tymbal/internal/driver"
)

// InputAccess describes the native microphone authorization state.
type InputAccess = driver.InputAccess

const (
	InputAccessGranted     = driver.InputAccessGranted
	InputAccessPrompt      = driver.InputAccessPrompt
	InputAccessDenied      = driver.InputAccessDenied
	InputAccessRestricted  = driver.InputAccessRestricted
	InputAccessUnavailable = driver.InputAccessUnavailable
)

// InputAccess queries native input authorization without opening a device or
// displaying a permission prompt. Hosts without an authorization API return
// granted; the operating system can still refuse a particular device at Open.
func (h Host) InputAccess() (InputAccess, error) {
	if h.d == nil {
		return InputAccessUnavailable, ErrState
	}
	if controller, ok := h.d.(driver.InputAccessController); ok {
		access, err := controller.InputAccess()
		return access, mapBackendError(err)
	}
	return InputAccessGranted, nil
}

// RequestInputAccess may display a system permission prompt. Call it only
// after the user asks to enable or record input, never during discovery.
// Canceling ctx stops this wait and never starts a stream.
func (h Host) RequestInputAccess(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if h.d == nil {
		return ErrState
	}
	if controller, ok := h.d.(driver.InputAccessController); ok {
		return mapBackendError(controller.RequestInputAccess(ctx))
	}
	return nil
}
