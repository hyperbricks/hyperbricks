package shared

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// GenerateTimestampRandomHash combines the current timestamp and a secure random number,
// hashes the combination using SHA-1, and returns the first 'length' hex characters.
func GenerateTimestampRandomHash(length int) string {
	// Get current UTC time with high precision.
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Generate 8 random bytes (64 bits).
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		// In production you might want to handle the error differently.
		panic(err)
	}
	randomPart := hex.EncodeToString(randomBytes) // 16 hex characters

	// Concatenate the timestamp and the random part.
	input := now + randomPart

	// Compute the SHA-1 hash of the combined input.
	hash := sha1.Sum([]byte(input))
	fullHex := hex.EncodeToString(hash[:]) // 40 hex characters

	// Ensure the requested length does not exceed the full hex string.
	if length > len(fullHex) {
		length = len(fullHex)
	}
	return fullHex[:length]
}

func GenerateCommentHash() string {
	return fmt.Sprintf("\n<!- %s ->\n", GenerateTimestampRandomHash(12))
}

// GenerateShortHash returns a short base62-encoded hash of the input string.
// 'bits' specifies how many bits of the SHA-256 digest to use. Default 48 bits is a good balance.
// Note: Using fewer bits increases collision risk; using more bits produces a longer string.
func GenerateShortHash(input string, bits int) string {
	// Validate bits: if not between 1 and 256, default to 48.
	if bits <= 0 || bits > 256 {
		bits = 48
	}

	// Compute SHA-256 hash of the input.
	hash := sha256.Sum256([]byte(input))

	// Convert the full hash to a big.Int.
	n := new(big.Int).SetBytes(hash[:])

	// We want only the top 'bits' bits, so we shift right.
	totalBits := 256
	if bits < totalBits {
		n.Rsh(n, uint(totalBits-bits))
	}

	// Encode the resulting number in base62.
	return base62Encode(n)
}

// base62Chars is our character set for base62 encoding.
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// base62Encode encodes a big.Int into a base62 string.
func base62Encode(n *big.Int) string {
	if n.Cmp(big.NewInt(0)) == 0 {
		return "0"
	}
	base := big.NewInt(62)
	result := ""
	mod := new(big.Int)
	zero := big.NewInt(0)
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		result = string(base62Chars[mod.Int64()]) + result
	}
	return result
}
