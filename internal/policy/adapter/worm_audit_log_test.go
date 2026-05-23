package adapter

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestHashChain_Links(t *testing.T) {
	var dbHashes []string
	record := func(content string) string {
		prev := ""
		if len(dbHashes) > 0 {
			prev = dbHashes[len(dbHashes)-1]
		}
		h := fmt.Sprintf("%x", sha256.Sum256([]byte(prev+content)))
		dbHashes = append(dbHashes, h)
		return h
	}

	h1 := record("event-1")
	h2 := record("event-2")
	h3 := record("event-3")

	expectedPrevForH2 := h1
	expectedH2 := fmt.Sprintf("%x", sha256.Sum256([]byte(h1+"event-2")))
	if h2 != expectedH2 {
		t.Errorf("h2 = %s, want %s", h2, expectedH2)
	}

	expectedH3 := fmt.Sprintf("%x", sha256.Sum256([]byte(h2+"event-3")))
	if h3 != expectedH3 {
		t.Errorf("h3 = %s, want %s\nh2 was: %s", h3, expectedH3, h2)
	}

	// Verify chaining
	if h1 == h2 || h2 == h3 {
		t.Fatal("all hashes identical — chain not working")
	}
	t.Logf("prev for h2 should be: %s", expectedPrevForH2)
}

func TestHashChain_Verify(t *testing.T) {
	chain := NewHashChain()
	h1 := chain.Next("e1")
	h2 := chain.Next("e2")

	if !chain.Verify("e1", "", h1) {
		t.Error("verify e1 failed")
	}
	if !chain.Verify("e2", h1, h2) {
		t.Error("verify e2 failed")
	}
	if chain.Verify("e2", "wrong-prev", h2) {
		t.Error("verify should reject wrong prevHash")
	}
}
