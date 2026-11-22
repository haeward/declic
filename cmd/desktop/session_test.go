package main

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo"
)

func TestCookieReadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	errResult := make(chan error, 1)
	go func() {
		_, err := readCookies(ctx, func() ([]mygo.Cookie, error) {
			close(started)
			<-release
			defer close(finished)
			return nil, nil
		})
		errResult <- err
	}()
	<-started
	cancel()
	if err := <-errResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked cookie reader must release UI on cancellation: %v", err)
	}
	close(release)
	<-finished
}
