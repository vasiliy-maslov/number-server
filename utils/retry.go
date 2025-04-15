package utils

import (
	"context"
	"fmt"
	"time"
)

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	for i := 0; i < attempts; i++ {
		if err := fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(sleep)
		}
	}
	return fmt.Errorf("failed after %d attempts", attempts)
}

func RetryContext(ctx context.Context, attempts int, sleep time.Duration, fn func() error) error {
	for i := 0; i < attempts; i++ {
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
	}
	return fmt.Errorf("failed after %d attempts", attempts)
}
