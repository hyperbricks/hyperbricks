package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// Test that the hash has the correct length.
func TestGenerateTimestampRandomHashLength(t *testing.T) {
	hash8 := shared.GenerateTimestampRandomHash(8)
	if len(hash8) != 8 {
		t.Errorf("Expected hash length of 8, got %d", len(hash8))
	}
	t.Logf("hash:%s", hash8)
	hash12 := shared.GenerateTimestampRandomHash(12)
	if len(hash12) != 12 {
		t.Errorf("Expected hash length of 12, got %d", len(hash12))
	}
	t.Logf("hash:%s", hash12)

}

// Test that successive calls produce different hashes (uniqueness test).
func TestGenerateTimestampRandomHashUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 100
	for i := 0; i < iterations; i++ {
		hash := shared.GenerateTimestampRandomHash(12)
		t.Logf("hash:%s", hash)
		if seen[hash] {
			t.Errorf("Hash collision detected: %s", hash)
		}

		seen[hash] = true
	}
}

// Test that if requested length exceeds the maximum available (SHA-1 produces 40 hex characters),
// the function returns a 40-character hash.
func TestGenerateTimestampRandomHashMaxLength(t *testing.T) {
	hash := shared.GenerateTimestampRandomHash(50)

	if len(hash) != 40 {
		t.Errorf("Expected hash length of 40 when requesting 50, got %d", len(hash))
	}
}

// // Test that the same input always returns the same hash.
// func TestGenerateShortHashConsistency(t *testing.T) {
// 	input := time.Now().UTC().Format(time.RFC3339Nano)
// 	hash1 := shared.GenerateShortHash(input, 48)
// 	hash2 := shared.GenerateShortHash(input, 48)
// 	if hash1 != hash2 {
// 		t.Errorf("Expected consistent hash for same input; got %s and %s", hash1, hash2)
// 	}
// }

// // Test that different inputs produce different hashes.
// func TestGenerateShortHashDifference(t *testing.T) {
// 	input1 := time.Now().UTC().Format(time.RFC3339Nano)
// 	input2 := time.Now().UTC().Format(time.RFC3339Nano)
// 	hash1 := shared.GenerateShortHash(input1, 48)
// 	hash2 := shared.GenerateShortHash(input2, 48)
// 	if hash1 == hash2 {
// 		t.Errorf("Expected different hashes for different inputs; both returned %s", hash1)
// 	}
// }

// // Test that increasing the number of bits produces a hash that is at least as long.
// func TestGenerateShortHashLength(t *testing.T) {
// 	ts := time.Now().UTC().Format(time.RFC3339Nano)
// 	hash48 := shared.GenerateShortHash(ts, 48)
// 	hash64 := shared.GenerateShortHash(ts, 64)
// 	if len(hash64) < len(hash48) {
// 		t.Errorf("Expected 64-bit hash to be longer than or equal to 48-bit hash; got lengths %d and %d", len(hash64), len(hash48))
// 	}
// 	// Optionally log the lengths:
// 	t.Logf("ts:%s 48-bit hash: %s (len=%d), 64-bit hash: %s (len=%d)", ts, hash48, len(hash48), hash64, len(hash64))
// }

// // Test that even an empty input produces a non-empty hash.
// func TestGenerateShortHashEmpty(t *testing.T) {
// 	input := ""
// 	hash := shared.GenerateShortHash(input, 48)
// 	if hash == "" {
// 		t.Error("Expected non-empty hash for empty input, got an empty string")
// 	}
// }
