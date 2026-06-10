package store

import (
	"strings"
	"testing"
)

func TestRandomPayloadTokenLength(t *testing.T) {
	for _, length := range []int{4, 6, 8} {
		token, err := randomPayloadToken(length)
		if err != nil {
			t.Fatal(err)
		}
		if len(token) != length {
			t.Fatalf("length %d: got %q (%d chars)", length, token, len(token))
		}
		for _, c := range token {
			if !strings.ContainsRune(payloadTokenChars, c) {
				t.Fatalf("invalid char %q in %q", c, token)
			}
		}
	}
}
