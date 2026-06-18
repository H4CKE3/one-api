package controller

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/songquanpeng/one-api/common/config"
	"gorm.io/gorm"
)

func TestCalculateRetryDelayUsesExponentialBackoff(t *testing.T) {
	originalBase := config.RetryBackoffBaseMilliseconds
	originalMax := config.RetryBackoffMaxMilliseconds
	originalJitter := config.RetryJitterMilliseconds
	defer func() {
		config.RetryBackoffBaseMilliseconds = originalBase
		config.RetryBackoffMaxMilliseconds = originalMax
		config.RetryJitterMilliseconds = originalJitter
	}()

	config.RetryBackoffBaseMilliseconds = 100
	config.RetryBackoffMaxMilliseconds = 1_000
	config.RetryJitterMilliseconds = 0

	delay := calculateRetryDelay(http.StatusTooManyRequests, 1, func(n int) int {
		return 0
	})
	if delay != 100*time.Millisecond {
		t.Fatalf("expected 100ms, got %v", delay)
	}

	delay = calculateRetryDelay(http.StatusTooManyRequests, 3, func(n int) int {
		return 0
	})
	if delay != 400*time.Millisecond {
		t.Fatalf("expected 400ms, got %v", delay)
	}
}

func TestCalculateRetryDelayCapsAndAddsJitter(t *testing.T) {
	originalBase := config.RetryBackoffBaseMilliseconds
	originalMax := config.RetryBackoffMaxMilliseconds
	originalJitter := config.RetryJitterMilliseconds
	defer func() {
		config.RetryBackoffBaseMilliseconds = originalBase
		config.RetryBackoffMaxMilliseconds = originalMax
		config.RetryJitterMilliseconds = originalJitter
	}()

	config.RetryBackoffBaseMilliseconds = 500
	config.RetryBackoffMaxMilliseconds = 600
	config.RetryJitterMilliseconds = 50

	delay := calculateRetryDelay(http.StatusTooManyRequests, 4, func(n int) int {
		return n - 1
	})
	if delay != 649*time.Millisecond {
		t.Fatalf("expected 649ms, got %v", delay)
	}
}

func TestCalculateRetryDelaySkipsNonRetryableStatus(t *testing.T) {
	delay := calculateRetryDelay(http.StatusBadRequest, 1, func(n int) int {
		return 0
	})
	if delay != 0 {
		t.Fatalf("expected zero delay, got %v", delay)
	}
}

func TestChooseRetryChannelIDPrefersFreshCandidate(t *testing.T) {
	channelID, err := chooseRetryChannelID(3, nil, 2, []int{1, 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if channelID != 3 {
		t.Fatalf("expected channel 3, got %d", channelID)
	}
}

func TestChooseRetryChannelIDFallsBackToPreviousFailedChannel(t *testing.T) {
	channelID, err := chooseRetryChannelID(0, errors.New("all channels cooling"), 2, []int{1, 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if channelID != 1 {
		t.Fatalf("expected channel 1, got %d", channelID)
	}
}

func TestChooseRetryChannelIDAlternatesWithDuplicateHistory(t *testing.T) {
	channelID, err := chooseRetryChannelID(1, nil, 1, []int{1, 2, 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if channelID != 2 {
		t.Fatalf("expected channel 2, got %d", channelID)
	}
}

func TestChooseRetryChannelIDReturnsSelectionErrorWithoutFallback(t *testing.T) {
	expectedErr := gorm.ErrRecordNotFound
	channelID, err := chooseRetryChannelID(0, expectedErr, 1, []int{1})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if channelID != 0 {
		t.Fatalf("expected no channel, got %d", channelID)
	}
}
