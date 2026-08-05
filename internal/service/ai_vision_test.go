package service

import (
	"errors"
	"testing"
	"time"
)

func TestRetryExtractSucceedsAfterTransientFailures(t *testing.T) {
	attempts := 0
	_, err := retryExtract(func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("segmen belum ada")
		}
		return "clip.mp4", nil
	}, time.Millisecond, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryExtractGivesUpAfterDeadline(t *testing.T) {
	attempts := 0
	_, err := retryExtract(func() (string, error) {
		attempts++
		return "", errors.New("segmen belum ada")
	}, time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected error after deadline exceeded, got nil")
	}
	if attempts < 2 {
		t.Fatalf("expected at least 2 attempts before giving up, got %d", attempts)
	}
}
