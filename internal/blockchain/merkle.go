package blockchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
)

// MerkleTree represents a Merkle tree structure
type MerkleTree struct {
	Root   *MerkleNode
	Leaves []*MerkleNode
}

// MerkleNode represents a node in the Merkle tree
type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Hash  Hash
	Data  []byte // Only set for leaf nodes
	IsLeaf bool
}

// NewMerkleTree creates a new Merkle tree from data items
func NewMerkleTree(data [][]byte) (*MerkleTree, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot create Merkle tree from empty data")
	}

	// Create leaf nodes
	leaves := make([]*MerkleNode, len(data))
	for i, item := range data {
		hash := sha256.Sum256(item)
		leaves[i] = &MerkleNode{
			Hash:   hash,
			Data:   item,
			IsLeaf: true,
		}
	}

	// Build tree
	root := buildTree(leaves)

	return &MerkleTree{
		Root:   root,
		Leaves: leaves,
	}, nil
}

// NewMerkleTreeFromHashes creates a Merkle tree from hashes
func NewMerkleTreeFromHashes(hashes []Hash) (*MerkleTree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("cannot create Merkle tree from empty hashes")
	}

	// Create leaf nodes
	leaves := make([]*MerkleNode, len(hashes))
	for i, hash := range hashes {
		leaves[i] = &MerkleNode{
			Hash:   hash,
			IsLeaf: true,
		}
	}

	// Build tree
	root := buildTree(leaves)

	return &MerkleTree{
		Root:   root,
		Leaves: leaves,
	}, nil
}

// buildTree recursively builds the Merkle tree
func buildTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 0 {
		return nil
	}

	if len(nodes) == 1 {
		return nodes[0]
	}

	// If odd number of nodes, duplicate the last one
	if len(nodes)%2 == 1 {
		nodes = append(nodes, nodes[len(nodes)-1])
	}

	// Build next level
	parentLevel := make([]*MerkleNode, len(nodes)/2)
	for i := 0; i < len(nodes); i += 2 {
		parentLevel[i/2] = &MerkleNode{
			Left:  nodes[i],
			Right: nodes[i+1],
			Hash:  hashPair(nodes[i].Hash, nodes[i+1].Hash),
		}
	}

	return buildTree(parentLevel)
}

// hashPair hashes two hashes together
func hashPair(left, right Hash) Hash {
	concat := append(left[:], right[:]...)
	hash := sha256.Sum256(concat)
	return hash
}

// GetRoot returns the Merkle root hash
func (mt *MerkleTree) GetRoot() Hash {
	if mt.Root == nil {
		return Hash{}
	}
	return mt.Root.Hash
}

// GetProof generates a Merkle proof for the data at the given index
func (mt *MerkleTree) GetProof(index int) (*MerkleProof, error) {
	if index < 0 || index >= len(mt.Leaves) {
		return nil, errors.New("index out of range")
	}

	proof := &MerkleProof{
		Index:  index,
		Leaf:   mt.Leaves[index].Hash,
		Hashes: []Hash{},
		Flags:  []bool{},
	}

	// Build proof by traversing up the tree
	currentLevel := make([]*MerkleNode, len(mt.Leaves))
	copy(currentLevel, mt.Leaves)

	for len(currentLevel) > 1 {
		// If odd number, duplicate last
		if len(currentLevel)%2 == 1 {
			currentLevel = append(currentLevel, currentLevel[len(currentLevel)-1])
		}

		// Find sibling
		if index%2 == 0 {
			// Current is left, sibling is right
			proof.Hashes = append(proof.Hashes, currentLevel[index+1].Hash)
			proof.Flags = append(proof.Flags, true) // true = sibling is on the right
		} else {
			// Current is right, sibling is left
			proof.Hashes = append(proof.Hashes, currentLevel[index-1].Hash)
			proof.Flags = append(proof.Flags, false) // false = sibling is on the left
		}

		// Move to parent level
		parentLevel := make([]*MerkleNode, len(currentLevel)/2)
		for i := 0; i < len(currentLevel); i += 2 {
			parentLevel[i/2] = &MerkleNode{
				Left:  currentLevel[i],
				Right: currentLevel[i+1],
				Hash:  hashPair(currentLevel[i].Hash, currentLevel[i+1].Hash),
			}
		}

		currentLevel = parentLevel
		index = index / 2
	}

	return proof, nil
}

// MerkleProof represents a Merkle proof
type MerkleProof struct {
	Index  int
	Leaf   Hash
	Hashes []Hash
	Flags  []bool // true = sibling is right, false = sibling is left
}

// Verify verifies the Merkle proof against the given root
func (mp *MerkleProof) Verify(root Hash) bool {
	hash := mp.Leaf

	for i, siblingHash := range mp.Hashes {
		if mp.Flags[i] {
			// Sibling is on the right
			hash = hashPair(hash, siblingHash)
		} else {
			// Sibling is on the left
			hash = hashPair(siblingHash, hash)
		}
	}

	return bytes.Equal(hash[:], root[:])
}

// Serialize serializes the Merkle proof
func (mp *MerkleProof) Serialize() []byte {
	buf := new(bytes.Buffer)

	// Write index
	buf.Write([]byte{byte(mp.Index >> 24), byte(mp.Index >> 16), byte(mp.Index >> 8), byte(mp.Index)})

	// Write leaf hash
	buf.Write(mp.Leaf[:])

	// Write number of hashes
	buf.Write([]byte{byte(len(mp.Hashes))})

	// Write hashes
	for _, h := range mp.Hashes {
		buf.Write(h[:])
	}

	// Write flags
	flagByte := byte(0)
	for i, flag := range mp.Flags {
		if flag {
			flagByte |= 1 << uint(i%8)
		}
		if (i+1)%8 == 0 || i == len(mp.Flags)-1 {
			buf.Write([]byte{flagByte})
			flagByte = 0
		}
	}

	return buf.Bytes()
}

// MerkleTreeFromTransactions creates a Merkle tree from transactions
func MerkleTreeFromTransactions(txs []*Transaction) (*MerkleTree, error) {
	if len(txs) == 0 {
		return nil, errors.New("no transactions")
	}

	hashes := make([]Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.CalculateHash()
	}

	return NewMerkleTreeFromHashes(hashes)
}

// VerifyTransaction verifies if a transaction is included in the Merkle tree
func (mt *MerkleTree) VerifyTransaction(tx *Transaction) bool {
	txHash := tx.CalculateHash()

	for _, leaf := range mt.Leaves {
		if bytes.Equal(leaf.Hash[:], txHash[:]) {
			return true
		}
	}

	return false
}

// GetLeafCount returns the number of leaves in the tree
func (mt *MerkleTree) GetLeafCount() int {
	return len(mt.Leaves)
}

// GetLeafHash returns the hash of the leaf at the given index
func (mt *MerkleTree) GetLeafHash(index int) (Hash, error) {
	if index < 0 || index >= len(mt.Leaves) {
		return Hash{}, errors.New("index out of range")
	}
	return mt.Leaves[index].Hash, nil
}
