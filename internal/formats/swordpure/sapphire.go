// sapphire.go implements the Sapphire II stream cipher used by SWORD modules.
//
// The Sapphire II Stream Cipher was designed by Michael Paul Johnson in 1995.
// It is a symmetric stream cipher that uses a key-dependent permutation table.
// SWORD modules use this cipher for content protection.
//
// Technical details:
// - State consists of 5 index values (rotor, ratchet, avalanche, lastPlain, lastCipher)
// - Plus a 256-byte permutation vector (cards)
// - Key setup shuffles the permutation based on key bytes
// - Each byte is encrypted using the current permutation state
// - State is updated after each byte using plaintext and ciphertext feedback
//
// References:
// - Original paper: https://cryptography.org/mpj/sapphire.pdf
// - SWORD implementation: https://www.crosswire.org/jsword/javadoc/org/crosswire/common/crypt/Sapphire.html
package swordpure

// Decryptor is the interface for SWORD module decryption.
// This allows for future cipher implementations while maintaining
// compatibility with existing Sapphire II encrypted modules.
type Decryptor interface {
	// Decrypt decrypts data in place.
	Decrypt(data []byte)

	// DecryptCopy decrypts data and returns a new slice.
	DecryptCopy(data []byte) []byte
}

// CipherType identifies the encryption algorithm used.
type CipherType string

const (
	// CipherSapphire is the Sapphire II stream cipher (SWORD default).
	CipherSapphire CipherType = "Sapphire"

	// CipherNone indicates no encryption.
	CipherNone CipherType = ""
)

// NewDecryptor creates a Decryptor for the given cipher type and key.
// Currently only Sapphire II is supported (the only cipher used by SWORD).
// This function provides a pluggable interface for future cipher support.
func NewDecryptor(cipherType CipherType, key []byte) Decryptor {
	switch cipherType {
	case CipherSapphire:
		return NewSapphireCipher(key)
	case CipherNone:
		return &nullDecryptor{}
	default:
		// Unknown cipher type - fall back to Sapphire (SWORD default)
		return NewSapphireCipher(key)
	}
}

// nullDecryptor is a no-op decryptor for unencrypted modules.
type nullDecryptor struct{}

func (n *nullDecryptor) Decrypt(data []byte) {}

func (n *nullDecryptor) DecryptCopy(data []byte) []byte {
	result := make([]byte, len(data))
	copy(result, data)
	return result
}

// SapphireCipher implements the Sapphire II stream cipher.
type SapphireCipher struct {
	cards      [256]byte
	rotor      byte
	ratchet    byte
	avalanche  byte
	lastPlain  byte
	lastCipher byte
}

// NewSapphireCipher creates a new Sapphire II cipher initialized with the given key.
// If key is empty, the cipher is initialized to an identity state.
func NewSapphireCipher(key []byte) *SapphireCipher {
	s := &SapphireCipher{}
	s.initialize(key)
	return s
}

// initialize sets up the cipher state from the key.
// This matches JSword's initialization exactly.
func (s *SapphireCipher) initialize(key []byte) {
	// Start with cards all in order, one of each
	for i := range s.cards {
		s.cards[i] = byte(i)
	}

	if len(key) == 0 {
		// For empty key, use identity permutation with fixed initial state
		s.rotor = 1
		s.ratchet = 3
		s.avalanche = 5
		s.lastPlain = 7
		s.lastCipher = 11
		return
	}

	// Key scheduling - swap the card at each position with some other card
	var keyPos int
	var rsum byte

	for i := 255; i >= 0; i-- {
		toSwap := s.keyRand(byte(i), key, &keyPos, &rsum)
		s.cards[i], s.cards[toSwap] = s.cards[toSwap], s.cards[i]
	}

	// Initialize the indices and data dependencies
	s.rotor = s.cards[1]
	s.ratchet = s.cards[3]
	s.avalanche = s.cards[5]
	s.lastPlain = s.cards[7]
	s.lastCipher = s.cards[rsum] // JSword uses cards[rsum], not cards[ratchet]
}

// keyRand generates a random value for key scheduling.
// This matches JSword's keyrand() exactly.
func (s *SapphireCipher) keyRand(limit byte, key []byte, keyPos *int, rsum *byte) byte {
	if limit == 0 {
		return 0
	}

	retryLimiter := 0
	mask := byte(1)

	// Build mask that covers limit
	for mask < limit {
		mask = (mask << 1) + 1
	}

	var u byte
	for {
		// Update rsum with cards[rsum] + key[keyPos]
		*rsum = s.cards[*rsum] + key[*keyPos]
		*keyPos++
		if *keyPos >= len(key) {
			*keyPos = 0
			*rsum += byte(len(key))
		}

		u = mask & *rsum

		retryLimiter++
		if retryLimiter > 11 {
			u = u % (limit + 1)
			return u
		}

		if u <= limit {
			return u
		}
	}
}

// Decrypt decrypts a byte slice in place.
func (s *SapphireCipher) Decrypt(data []byte) {
	for i := range data {
		data[i] = s.decryptByte(data[i])
	}
}

// DecryptCopy decrypts a byte slice and returns a new slice.
func (s *SapphireCipher) DecryptCopy(data []byte) []byte {
	result := make([]byte, len(data))
	copy(result, data)
	s.Decrypt(result)
	return result
}

// decryptByte decrypts a single byte using the same cipher operation.
// Sapphire II uses symmetric cipher operation - decryption swaps lastPlain/lastCipher assignments.
func (s *SapphireCipher) decryptByte(b byte) byte {
	// Advance rotor and ratchet
	s.ratchet += s.cards[s.rotor]
	s.rotor++

	// 4-way circular swap of cards
	swapTemp := s.cards[s.lastCipher]
	s.cards[s.lastCipher] = s.cards[s.ratchet]
	s.cards[s.ratchet] = s.cards[s.lastPlain]
	s.cards[s.lastPlain] = s.cards[s.rotor]
	s.cards[s.rotor] = swapTemp

	// Update avalanche with the swapped value
	s.avalanche += s.cards[swapTemp]

	// Get output from card permutation (complex nonlinear transformation)
	// output = cards[(cards[ratchet] + cards[rotor])] XOR
	//          cards[cards[(cards[lastPlain] + cards[lastCipher] + cards[avalanche])]]
	idx1 := s.cards[s.ratchet] + s.cards[s.rotor]
	idx2 := s.cards[s.lastPlain] + s.cards[s.lastCipher] + s.cards[s.avalanche]
	output := s.cards[idx1] ^ s.cards[s.cards[idx2]]

	// Decrypt: XOR input with output, store for feedback
	s.lastPlain = b ^ output
	s.lastCipher = b

	return s.lastPlain
}

// Encrypt encrypts a byte slice in place.
func (s *SapphireCipher) Encrypt(data []byte) {
	for i := range data {
		data[i] = s.encryptByte(data[i])
	}
}

// encryptByte encrypts a single byte.
// Same as decrypt but swaps lastPlain/lastCipher at the end.
func (s *SapphireCipher) encryptByte(b byte) byte {
	// Advance rotor and ratchet
	s.ratchet += s.cards[s.rotor]
	s.rotor++

	// 4-way circular swap of cards
	swapTemp := s.cards[s.lastCipher]
	s.cards[s.lastCipher] = s.cards[s.ratchet]
	s.cards[s.ratchet] = s.cards[s.lastPlain]
	s.cards[s.lastPlain] = s.cards[s.rotor]
	s.cards[s.rotor] = swapTemp

	// Update avalanche with the swapped value
	s.avalanche += s.cards[swapTemp]

	// Get output from card permutation
	idx1 := s.cards[s.ratchet] + s.cards[s.rotor]
	idx2 := s.cards[s.lastPlain] + s.cards[s.lastCipher] + s.cards[s.avalanche]
	output := s.cards[idx1] ^ s.cards[s.cards[idx2]]

	// Encrypt: XOR input with output, store for feedback (swapped from decrypt)
	s.lastCipher = b ^ output
	s.lastPlain = b

	return s.lastCipher
}

// Reset re-initializes the cipher with a new key.
func (s *SapphireCipher) Reset(key []byte) {
	s.initialize(key)
}
