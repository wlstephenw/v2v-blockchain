package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// Hash represents a 32-byte hash
type Hash [32]byte

// Bytes returns the hash as a byte slice
func (h Hash) Bytes() []byte {
	return h[:]
}

// String returns the hash as a hex string
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// HexToHash converts a hex string to Hash
func HexToHash(s string) (Hash, error) {
	var h Hash
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, err
	}
	if len(b) != 32 {
		return h, fmt.Errorf("invalid hash length: %d", len(b))
	}
	copy(h[:], b)
	return h, nil
}

// Address represents a 20-byte address
type Address [20]byte

// Bytes returns the address as a byte slice
func (a Address) Bytes() []byte {
	return a[:]
}

// String returns the address as a hex string
func (a Address) String() string {
	return hex.EncodeToString(a[:])
}

// HexToAddress converts a hex string to Address
func HexToAddress(s string) (Address, error) {
	var a Address
	b, err := hex.DecodeString(s)
	if err != nil {
		return a, err
	}
	if len(b) != 20 {
		return a, fmt.Errorf("invalid address length: %d", len(b))
	}
	copy(a[:], b)
	return a, nil
}

// BlockHeader represents the header of a block
type BlockHeader struct {
	Hash       Hash      `json:"hash"`        // 32 bytes - Block hash
	PrevHash   Hash      `json:"prev_hash"`   // 32 bytes - Previous block hash
	Timestamp  int64     `json:"timestamp"`   // 8 bytes - Unix timestamp
	MerkleRoot Hash      `json:"merkle_root"` // 32 bytes - Merkle root of transactions
	Validator  Address   `json:"validator"`   // 20 bytes - Validator address
	Signature  []byte    `json:"signature"`   // 65 bytes - Block signature
	TxCount    uint32    `json:"tx_count"`    // 4 bytes - Transaction count
	Height     uint64    `json:"height"`      // 8 bytes - Block height
	Nonce      uint64    `json:"nonce"`       // 8 bytes - Nonce for mining (if needed)
}

// Block represents a complete block with header and transactions
type Block struct {
	Header       *BlockHeader  `json:"header"`
	Transactions []*Transaction `json:"transactions"`
}

// Transaction represents a blockchain transaction
type Transaction struct {
	Hash      Hash    `json:"hash"`       // Transaction hash
	From      Address `json:"from"`       // Sender address
	To        Address `json:"to"`         // Recipient address (optional for some tx types)
	Type      TxType  `json:"type"`       // Transaction type
	Nonce     uint64  `json:"nonce"`      // Transaction nonce
	GasPrice  uint64  `json:"gas_price"`  // Gas price
	GasLimit  uint64  `json:"gas_limit"`  // Gas limit
	Value     uint64  `json:"value"`      // Transaction value
	Data      []byte  `json:"data"`       // Transaction data/payload
	Signature []byte  `json:"signature"`  // Transaction signature
	Timestamp int64   `json:"timestamp"`  // Transaction timestamp
}

// TxType represents the type of transaction
type TxType uint8

const (
	TxTypeTransfer TxType = iota // Regular transfer
	TxTypeRegister               // Vehicle registration
	TxTypePlatoonCreate          // Create platoon
	TxTypePlatoonJoin            // Join platoon
	TxTypePlatoonLeave           // Leave platoon
	TxTypePlatoonDissolve        // Dissolve platoon
	TxTypeValidatorAdd           // Add validator
	TxTypeValidatorRemove        // Remove validator
	TxTypeCertificateUpdate      // Update certificate
	TxTypeMessage                // V2V message
)

// String returns the string representation of TxType
func (t TxType) String() string {
	switch t {
	case TxTypeTransfer:
		return "transfer"
	case TxTypeRegister:
		return "register"
	case TxTypePlatoonCreate:
		return "platoon_create"
	case TxTypePlatoonJoin:
		return "platoon_join"
	case TxTypePlatoonLeave:
		return "platoon_leave"
	case TxTypePlatoonDissolve:
		return "platoon_dissolve"
	case TxTypeValidatorAdd:
		return "validator_add"
	case TxTypeValidatorRemove:
		return "validator_remove"
	case TxTypeCertificateUpdate:
		return "certificate_update"
	case TxTypeMessage:
		return "message"
	default:
		return "unknown"
	}
}

// CalculateHash calculates the hash of the block header
func (h *BlockHeader) CalculateHash() Hash {
	data := h.SerializeWithoutHash()
	hash := crypto.Keccak256(data)
	var result Hash
	copy(result[:], hash)
	return result
}

// SerializeWithoutHash serializes the header without the hash field
func (h *BlockHeader) SerializeWithoutHash() []byte {
	buf := new(bytes.Buffer)

	// Write fields in order (excluding hash)
	buf.Write(h.PrevHash[:])
	binary.Write(buf, binary.BigEndian, h.Timestamp)
	buf.Write(h.MerkleRoot[:])
	buf.Write(h.Validator[:])
	buf.Write(h.Signature)
	binary.Write(buf, binary.BigEndian, h.TxCount)
	binary.Write(buf, binary.BigEndian, h.Height)
	binary.Write(buf, binary.BigEndian, h.Nonce)

	return buf.Bytes()
}

// Serialize serializes the complete header
func (h *BlockHeader) Serialize() []byte {
	buf := new(bytes.Buffer)
	buf.Write(h.Hash[:])
	buf.Write(h.SerializeWithoutHash())
	return buf.Bytes()
}

// Deserialize deserializes bytes into a BlockHeader
func (h *BlockHeader) Deserialize(data []byte) error {
	if len(data) < 32 {
		return fmt.Errorf("insufficient data for header deserialization")
	}

	copy(h.Hash[:], data[0:32])
	copy(h.PrevHash[:], data[32:64])
	h.Timestamp = int64(binary.BigEndian.Uint64(data[64:72]))
	copy(h.MerkleRoot[:], data[72:104])
	copy(h.Validator[:], data[104:124])

	// Signature length is variable, but typically 65 bytes
	sigLen := len(data) - 124 - 20 // 20 = 4 + 8 + 8 for TxCount, Height, Nonce
	if sigLen < 0 {
		return fmt.Errorf("invalid data length")
	}
	h.Signature = make([]byte, sigLen)
	copy(h.Signature, data[124:124+sigLen])

	offset := 124 + sigLen
	h.TxCount = binary.BigEndian.Uint32(data[offset : offset+4])
	h.Height = binary.BigEndian.Uint64(data[offset+4 : offset+12])
	h.Nonce = binary.BigEndian.Uint64(data[offset+12 : offset+20])

	return nil
}

// NewBlock creates a new block
func NewBlock(height uint64, prevHash Hash, validator Address, txs []*Transaction) *Block {
	block := &Block{
		Header: &BlockHeader{
			PrevHash:  prevHash,
			Height:    height,
			Validator: validator,
			TxCount:   uint32(len(txs)),
			Timestamp: time.Now().Unix(),
		},
		Transactions: txs,
	}

	// Calculate Merkle root
	block.Header.MerkleRoot = CalculateMerkleRoot(txs)

	// Calculate block hash
	block.Header.Hash = block.Header.CalculateHash()

	return block
}

// NewGenesisBlock creates the genesis block
func NewGenesisBlock(validator Address) *Block {
	return NewBlock(0, Hash{}, validator, []*Transaction{})
}

// Sign signs the block with the given private key
func (b *Block) Sign(privKey []byte) error {
	data := b.Header.SerializeWithoutHash()
	hash := crypto.Keccak256Hash(data)

	// Convert byte slice to ECDSA private key
	privateKey, err := crypto.ToECDSA(privKey)
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return err
	}
	b.Header.Signature = sig
	// Recalculate hash after signing
	b.Header.Hash = b.Header.CalculateHash()
	return nil
}

// VerifySignature verifies the block signature
func (b *Block) VerifySignature() bool {
	if len(b.Header.Signature) == 0 {
		return false
	}

	data := b.Header.SerializeWithoutHash()
	hash := crypto.Keccak256Hash(data)

	// Recover public key from signature
	pubKey, err := crypto.SigToPub(hash.Bytes(), b.Header.Signature)
	if err != nil {
		return false
	}

	// Verify validator matches recovered public key
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return bytes.Equal(b.Header.Validator[:], recoveredAddr[:])
}

// CalculateMerkleRoot calculates the Merkle root of transactions
func CalculateMerkleRoot(txs []*Transaction) Hash {
	if len(txs) == 0 {
		return Hash{}
	}

	// Calculate transaction hashes
	hashes := make([][]byte, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.CalculateHash().Bytes()
	}

	// Build Merkle tree
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		newLevel := make([][]byte, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			concat := append(hashes[i], hashes[i+1]...)
			hash := sha256.Sum256(concat)
			newLevel[i/2] = hash[:]
		}
		hashes = newLevel
	}

	var result Hash
	copy(result[:], hashes[0])
	return result
}

// CalculateHash calculates the transaction hash
func (tx *Transaction) CalculateHash() Hash {
	data := tx.Serialize()
	hash := crypto.Keccak256(data)
	var result Hash
	copy(result[:], hash)
	return result
}

// Serialize serializes the transaction
func (tx *Transaction) Serialize() []byte {
	buf := new(bytes.Buffer)

	buf.Write(tx.From[:])
	buf.Write(tx.To[:])
	binary.Write(buf, binary.BigEndian, tx.Type)
	binary.Write(buf, binary.BigEndian, tx.Nonce)
	binary.Write(buf, binary.BigEndian, tx.GasPrice)
	binary.Write(buf, binary.BigEndian, tx.GasLimit)
	binary.Write(buf, binary.BigEndian, tx.Value)
	binary.Write(buf, binary.BigEndian, uint32(len(tx.Data)))
	buf.Write(tx.Data)
	binary.Write(buf, binary.BigEndian, tx.Timestamp)

	return buf.Bytes()
}

// Sign signs the transaction with the given private key
func (tx *Transaction) Sign(privKey []byte) error {
	tx.Hash = tx.CalculateHash()
	data := tx.Hash.Bytes()

	// Convert byte slice to ECDSA private key
	privateKey, err := crypto.ToECDSA(privKey)
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(data, privateKey)
	if err != nil {
		return err
	}
	tx.Signature = sig
	return nil
}

// VerifySignature verifies the transaction signature
func (tx *Transaction) VerifySignature() bool {
	if len(tx.Signature) == 0 {
		return false
	}

	hash := tx.CalculateHash()
	pubKey, err := crypto.SigToPub(hash.Bytes(), tx.Signature)
	if err != nil {
		return false
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return bytes.Equal(tx.From[:], recoveredAddr[:])
}

// MarshalJSON implements json.Marshaler
func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (h *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := HexToHash(s)
	if err != nil {
		return err
	}
	*h = parsed
	return nil
}

// MarshalJSON implements json.Marshaler
func (a Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := HexToAddress(s)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// BlockSize returns the approximate size of the block in bytes
func (b *Block) BlockSize() int {
	size := len(b.Header.Serialize())
	for _, tx := range b.Transactions {
		size += len(tx.Serialize()) + len(tx.Signature)
	}
	return size
}

// IsGenesis checks if this is the genesis block
func (b *Block) IsGenesis() bool {
	return b.Header.Height == 0
}
