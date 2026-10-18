package tymbal_test

import (
	"context"
	"errors"
	"testing"

	"m31labs.dev/tymbal"
)

func TestInputAccessInvalidAndPortableHosts(t *testing.T) {
	var invalid tymbal.Host
	if access, err := invalid.InputAccess(); access != tymbal.InputAccessUnavailable || !errors.Is(err, tymbal.ErrState) {
		t.Fatalf("invalid host authorization: %q %v", access, err)
	}
	if err := invalid.RequestInputAccess(context.Background()); !errors.Is(err, tymbal.ErrState) {
		t.Fatal(err)
	}
	host, _ := tymbal.NewFakeHost(tymbal.FakeConfig{})
	if access, err := host.InputAccess(); err != nil || access != tymbal.InputAccessGranted {
		t.Fatalf("portable host authorization: %q %v", access, err)
	}
	if err := host.RequestInputAccess(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := host.RequestInputAccess(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
