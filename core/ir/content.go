package ir

import (
	"crypto/sha256"
	"encoding/hex"
)

// content.go - Content block utility functions
// Note: Type definitions are in types.go

// ComputeHash calculates and stores the SHA-256 hash of the Text field.
func (cb *ContentBlock) ComputeHash() string {
	h := sha256.Sum256([]byte(cb.Text))
	cb.Hash = hex.EncodeToString(h[:])
	return cb.Hash
}

// VerifyHash returns true if the stored hash matches the computed hash.
func (cb *ContentBlock) VerifyHash() bool {
	if cb.Hash == "" {
		return false
	}
	h := sha256.Sum256([]byte(cb.Text))
	return cb.Hash == hex.EncodeToString(h[:])
}

// isWhitespaceByte reports whether c is an ASCII whitespace character.
func isWhitespaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isWordByte reports whether c belongs to a word token.
// Words consist of ASCII letters, digits, apostrophes, and non-ASCII bytes
// (the latter to preserve multi-byte UTF-8 sequences as word characters).
func isWordByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '\'' || c >= 0x80
}

// classifyByte returns the TokenType for a single byte.
func classifyByte(c byte) TokenType {
	if isWhitespaceByte(c) {
		return TokenWhitespace
	}
	if isWordByte(c) {
		return TokenWord
	}
	return TokenPunctuation
}

// appendToken appends a completed token to tokens and returns the updated slice
// and a reset byte buffer. It is a no-op when buf is empty.
func appendToken(tokens []*Token, buf []byte, index, start, end int, typ TokenType) ([]*Token, int) {
	if len(buf) == 0 {
		return tokens, index
	}
	tokens = append(tokens, &Token{
		ID:        "",
		Index:     index,
		CharStart: start,
		CharEnd:   end,
		Text:      string(buf),
		Type:      typ,
	})
	return tokens, index + 1
}

// Tokenize breaks text into tokens. This is a simple implementation
// that handles common English/Western text patterns.
func Tokenize(text string) []*Token {
	var tokens []*Token
	var tokenStart int
	var tokenText []byte
	var currentType TokenType
	index := 0

	for i := 0; i < len(text); i++ {
		c := text[i]
		newType := classifyByte(c)

		if len(tokenText) == 0 {
			tokenStart = i
			currentType = newType
		} else if newType != currentType {
			tokens, index = appendToken(tokens, tokenText, index, tokenStart, i, currentType)
			tokenText = nil
			tokenStart = i
			currentType = newType
		}

		tokenText = append(tokenText, c)
	}

	tokens, _ = appendToken(tokens, tokenText, index, tokenStart, len(text), currentType)
	return tokens
}
