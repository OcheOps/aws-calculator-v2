package auth

import "testing"

func TestGenerateToken_UniqueAnd32Bytes(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, _ := GenerateToken()
	if a == b {
		t.Error("expected unique tokens")
	}
	if len(a) < 40 {
		t.Errorf("token too short: %d", len(a))
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := HashToken("abc")
	h2 := HashToken("abc")
	if h1 != h2 {
		t.Errorf("hash mismatch")
	}
	if HashToken("abd") == h1 {
		t.Error("collision")
	}
}
