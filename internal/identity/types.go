package identity

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
)

// IdentityStatus represents the status of a vehicle identity
type IdentityStatus uint8

const (
	IdentityStatusUnknown IdentityStatus = iota
	IdentityStatusActive
	IdentityStatusExpired
	IdentityStatusRevoked
	IdentityStatusPending
)

func (s IdentityStatus) String() string {
	switch s {
	case IdentityStatusActive:
		return "active"
	case IdentityStatusExpired:
		return "expired"
	case IdentityStatusRevoked:
		return "revoked"
	case IdentityStatusPending:
		return "pending"
	default:
		return "unknown"
	}
}

// VehicleIdentity represents a registered vehicle's identity
type VehicleIdentity struct {
	VehicleID       blockchain.Address `json:"vehicle_id"`       // Blockchain address derived from public key
	PublicKey       []byte             `json:"public_key"`       // ECDSA public key (uncompressed, 65 bytes)
	Certificate     []byte             `json:"certificate"`      // X.509 certificate PEM
	CertificateHash blockchain.Hash    `json:"certificate_hash"` // Hash of certificate for quick lookup
	FleetID         string             `json:"fleet_id,omitempty"` // Optional fleet ID for fleet management

	// Status and timestamps
	Status        IdentityStatus `json:"status"`
	RegisteredAt  int64          `json:"registered_at"`  // Unix timestamp
	ExpiresAt     int64          `json:"expires_at"`     // Certificate expiration
	LastRotatedAt int64          `json:"last_rotated_at"` // Last certificate rotation
	LastSeenAt    int64          `json:"last_seen_at"`   // Last activity timestamp

	// CA information
	CAName      string `json:"ca_name"`      // Issuing CA name
	CAChainHash []byte `json:"ca_chain_hash"` // Hash of CA certificate chain
}

// CertificateInfo holds parsed certificate information
type CertificateInfo struct {
	SerialNumber  string
	Subject       string
	Issuer        string
	NotBefore     time.Time
	NotAfter      time.Time
	DNSNames      []string
	EmailAddresses []string
	PublicKey     []byte
}

// IsExpired checks if the identity's certificate has expired
func (id *VehicleIdentity) IsExpired() bool {
	return time.Now().Unix() > id.ExpiresAt
}

// NeedsRotation checks if certificate needs rotation (default 7 days before expiration)
func (id *VehicleIdentity) NeedsRotation(daysBefore int) bool {
	if daysBefore <= 0 {
		daysBefore = 7 // Default 7 days
	}
	rotationThreshold := id.ExpiresAt - int64(daysBefore*24*60*60)
	return time.Now().Unix() >= rotationThreshold
}

// IsActive checks if the identity is active and valid
func (id *VehicleIdentity) IsActive() bool {
	return id.Status == IdentityStatusActive && !id.IsExpired()
}

// ParseCertificate parses the X.509 certificate
func (id *VehicleIdentity) ParseCertificate() (*x509.Certificate, error) {
	block, _ := pem.Decode(id.Certificate)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// GetCertificateInfo extracts certificate information
func (id *VehicleIdentity) GetCertificateInfo() (*CertificateInfo, error) {
	cert, err := id.ParseCertificate()
	if err != nil {
		return nil, err
	}

	return &CertificateInfo{
		SerialNumber:   cert.SerialNumber.String(),
		Subject:        cert.Subject.String(),
		Issuer:         cert.Issuer.String(),
		NotBefore:      cert.NotBefore,
		NotAfter:       cert.NotAfter,
		DNSNames:       cert.DNSNames,
		EmailAddresses: cert.EmailAddresses,
		PublicKey:      id.PublicKey,
	}, nil
}

// CRLEntry represents a certificate revocation list entry
type CRLEntry struct {
	SerialNumber string `json:"serial_number"`
	RevokedAt    int64  `json:"revoked_at"`
	Reason       string `json:"reason"`
}

// CertificateRevocationList represents the CRL
type CertificateRevocationList struct {
	IssuerHash  []byte      `json:"issuer_hash"`
	UpdatedAt   int64       `json:"updated_at"`
	NextUpdate  int64       `json:"next_update"`
	Entries     []CRLEntry  `json:"entries"`
	Signature   []byte      `json:"signature"`
}

// IsRevoked checks if a certificate serial number is in the CRL
func (crl *CertificateRevocationList) IsRevoked(serialNumber string) bool {
	for _, entry := range crl.Entries {
		if entry.SerialNumber == serialNumber {
			return true
		}
	}
	return false
}

// RegistrationRequest represents a vehicle registration request
type RegistrationRequest struct {
	VehicleID   blockchain.Address `json:"vehicle_id"`
	PublicKey   []byte             `json:"public_key"`
	Certificate []byte             `json:"certificate"`
	FleetID     string             `json:"fleet_id,omitempty"`
	Signature   []byte             `json:"signature"` // Self-signed with vehicle's private key
	Timestamp   int64              `json:"timestamp"`
}

// BatchRegistrationRequest represents a batch registration request for fleets
type BatchRegistrationRequest struct {
	FleetID   string                 `json:"fleet_id"`
	Requests  []RegistrationRequest  `json:"requests"`
	CASignature []byte               `json:"ca_signature"` // CA signs the batch
	Timestamp int64                  `json:"timestamp"`
}

// VerifyRequestSignature verifies the registration request signature
func (req *RegistrationRequest) VerifyRequestSignature() error {
	if len(req.Signature) == 0 {
		return fmt.Errorf("missing signature")
	}

	// Recover public key from signature
	data := append(req.VehicleID[:], req.PublicKey...)
	data = append(data, req.Certificate...)
	hash := crypto.Keccak256(data)

	pubKey, err := crypto.SigToPub(hash, req.Signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	var recoveredBytes [20]byte
	copy(recoveredBytes[:], recoveredAddr[:])
	if recoveredBytes != req.VehicleID {
		return fmt.Errorf("signature verification failed: address mismatch")
	}

	return nil
}

// OnlineStatus represents the online status of a node
type OnlineStatus struct {
	VehicleID    blockchain.Address `json:"vehicle_id"`
	IsOnline     bool               `json:"is_online"`
	LastSeenAt   int64              `json:"last_seen_at"`
	LastBlockAt  int64              `json:"last_block_at"` // Last block the node participated in
	NetworkAddr  string             `json:"network_addr,omitempty"`
}

// IdentityQueryResult represents the result of an identity query
type IdentityQueryResult struct {
	Identity   *VehicleIdentity `json:"identity"`
	Online     *OnlineStatus    `json:"online,omitempty"`
	Found      bool             `json:"found"`
}

// CertificateRotationRequest represents a certificate rotation request
type CertificateRotationRequest struct {
	VehicleID       blockchain.Address `json:"vehicle_id"`
	NewCertificate  []byte             `json:"new_certificate"`
	OldCertHash     blockchain.Hash    `json:"old_cert_hash"`
	Signature       []byte             `json:"signature"` // Signed with old private key
	Timestamp       int64              `json:"timestamp"`
}
