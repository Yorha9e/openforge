package domain

import (
	"strings"
	"sync"
	"testing"
)

// TestInMemoryEmbeddingIndex_TopKReturnsRelevant verifies that querying
// for "go language" returns doc1 first (which contains "go" and "language").
func TestInMemoryEmbeddingIndex_TopKReturnsRelevant(t *testing.T) {
	idx := NewInMemoryEmbeddingIndex()
	idx.Add("doc1", "go programming language")
	idx.Add("doc2", "python programming language")
	idx.Add("doc3", "rust systems language")

	hits := idx.Query("go language", 2)
	if len(hits) < 1 {
		t.Fatalf("expected at least 1 hit, got %d", len(hits))
	}
	if hits[0].ID != "doc1" {
		t.Errorf("expected doc1 as top hit, got %q", hits[0].ID)
	}
}

// TestInMemoryEmbeddingIndex_Empty verifies that querying an empty index
// returns no hits (not nil-related crashes).
func TestInMemoryEmbeddingIndex_Empty(t *testing.T) {
	idx := NewInMemoryEmbeddingIndex()
	hits := idx.Query("go language", 5)
	if len(hits) != 0 {
		t.Errorf("expected 0 hits on empty index, got %d", len(hits))
	}
}

// TestInMemoryEmbeddingIndex_TopKLimit verifies Query does not return
// more than topK hits.
func TestInMemoryEmbeddingIndex_TopKLimit(t *testing.T) {
	idx := NewInMemoryEmbeddingIndex()
	idx.Add("doc1", "go language")
	idx.Add("doc2", "go language")
	idx.Add("doc3", "go language")
	idx.Add("doc4", "go language")
	idx.Add("doc5", "go language")

	hits := idx.Query("go language", 2)
	if len(hits) > 2 {
		t.Errorf("expected at most 2 hits, got %d", len(hits))
	}
}

// TestInMemoryEmbeddingIndex_ThreadSafe exercises concurrent Add + Query
// to make sure the index does not panic or race. Run with -race.
func TestInMemoryEmbeddingIndex_ThreadSafe(t *testing.T) {
	idx := NewInMemoryEmbeddingIndex()

	// Seed with a few docs first.
	idx.Add("seed1", "go language programming")
	idx.Add("seed2", "python language programming")

	var wg sync.WaitGroup
	const workers = 8
	const opsPerWorker = 50

	for w := 0; w < workers; w++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				docID := id*opsPerWorker + i
				idx.Add("doc", toString(docID)+" go language text")
			}
		}(w)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				_ = idx.Query("go language", 5)
			}
		}(w)
	}

	wg.Wait()
}

// TestTokenize_Basic verifies the tokenize helper splits on whitespace
// and lowercases tokens.
func TestTokenize_Basic(t *testing.T) {
	tokens := tokenize("Hello World hello")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	for _, tok := range tokens {
		if tok != strings.ToLower(tok) {
			t.Errorf("token %q is not lowercase", tok)
		}
	}
}

func toString(n int) string {
	// Simple integer to string without importing strconv to keep test
	// dependencies minimal.
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
