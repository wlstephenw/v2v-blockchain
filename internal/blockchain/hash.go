package blockchain

import (
	"github.com/ethereum/go-ethereum/crypto"
)

// Keccak256 calculates the Keccak-256 hash of data
func Keccak256(data []byte) []byte {
	return crypto.Keccak256(data)
}

// Keccak256Hash calculates the Keccak-256 hash and returns it as a Hash type
func Keccak256Hash(data []byte) Hash {
	hash := crypto.Keccak256(data)
	var result Hash
	copy(result[:], hash)
	return result
}

// DoubleHash calculates double Keccak-256 hash
func DoubleHash(data []byte) Hash {
	first := crypto.Keccak256(data)
	second := crypto.Keccak256(first)
	var result Hash
	copy(result[:], second)
	return result
}

// HashData hashes arbitrary data using Keccak-256
func HashData(data []byte) Hash {
	return Keccak256Hash(data)
}

// HashItems hashes multiple byte slices together
func HashItems(items ...[]byte) Hash {
	var data []byte
	for _, item := range items {
		data = append(data, item...)
	}
	return Keccak256Hash(data)
}

// EmptyHash returns an empty hash (all zeros)
func EmptyHash() Hash {
	return Hash{}
}

// IsEmpty checks if the hash is empty (all zeros)
func (h Hash) IsEmpty() bool {
	return h == Hash{}
}

// Equals checks if two hashes are equal
func (h Hash) Equals(other Hash) bool {
	return h == other
}

// Xor performs XOR operation between two hashes
func (h Hash) Xor(other Hash) Hash {
	var result Hash
	for i := 0; i < 32; i++ {
		result[i] = h[i] ^ other[i]
	}
	return result
}
