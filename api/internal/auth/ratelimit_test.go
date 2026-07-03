package auth

import (
	"testing"
	"time"
)

func TestLimiter_AllowsThenBlocks(t *testing.T) {
	l := NewLimiter(2, time.Hour)
	if !l.Allow("k") {
		t.Fatal("first should pass")
	}
	if !l.Allow("k") {
		t.Fatal("second should pass")
	}
	if l.Allow("k") {
		t.Fatal("third should be blocked")
	}
	if !l.Allow("other-key") {
		t.Fatal("independent keys not isolated")
	}
}
