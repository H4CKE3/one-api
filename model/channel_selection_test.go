package model

import (
	"errors"
	"testing"
)

func TestSelectChannelIDSkipsRateLimitedCandidates(t *testing.T) {
	candidates := []channelCandidate{
		{channelID: 1, priority: 10},
		{channelID: 2, priority: 10},
		{channelID: 3, priority: 5},
	}

	channelID, err := selectChannelID(candidates, false, func(id int) bool {
		return id == 1
	}, func(n int) int {
		return 0
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if channelID != 2 {
		t.Fatalf("expected channel 2, got %d", channelID)
	}
}

func TestSelectChannelIDUsesLowerPriorityPoolOnRetry(t *testing.T) {
	candidates := []channelCandidate{
		{channelID: 1, priority: 10},
		{channelID: 2, priority: 10},
		{channelID: 3, priority: 5},
	}

	channelID, err := selectChannelID(candidates, true, func(id int) bool {
		return id == 1 || id == 2
	}, func(n int) int {
		return 0
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if channelID != 3 {
		t.Fatalf("expected channel 3, got %d", channelID)
	}
}

func TestSelectChannelIDReturnsCooldownErrorWhenAllCandidatesCooling(t *testing.T) {
	candidates := []channelCandidate{
		{channelID: 1, priority: 10},
		{channelID: 2, priority: 10},
	}

	_, err := selectChannelID(candidates, false, func(id int) bool {
		return true
	}, func(n int) int {
		return 0
	})
	if !errors.Is(err, ErrNoChannelAvailableWithinCooldown) {
		t.Fatalf("expected ErrNoChannelAvailableWithinCooldown, got %v", err)
	}
}
