package relay_utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const patTokenPrefix = "pat_"

func GeneratePatToken() (plain string, hash string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plain = patTokenPrefix + hex.EncodeToString(b[:])
	return plain, HashPatToken(plain), nil
}

func HashPatToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
