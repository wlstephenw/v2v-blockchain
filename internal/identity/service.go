package identity

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// Service manages vehicle identities and certificates
type Service struct {
	storage       storage.Storage
	crl           *CertificateRevocationList
	crlMu         sync.RWMutex

	// Cache for quick lookups
	identityCache map[blockchain.Address]*VehicleIdentity
	cacheMu       sync.RWMutex

	// Online status tracking
	onlineStatus  map[blockchain.Address]*OnlineStatus
	onlineMu      sync.RWMutex

	// CA certificate for verification
	caCert        *x509.Certificate
	caCertPool    *x509.CertPool

	// Certificate rotation settings
	rotateDaysBefore int

	// Lifecycle
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewService creates a new identity service
func NewService(store storage.Storage, caCertPEM []byte) (*Service, error) {
	svc := &Service{
		storage:          store,
		identityCache:    make(map[blockchain.Address]*VehicleIdentity),
		onlineStatus:     make(map[blockchain.Address]*OnlineStatus),
		stopCh:           make(chan struct{}),
		rotateDaysBefore: 7, // Default 7 days
		crl: &CertificateRevocationList{
			Entries: make([]CRLEntry, 0),
		},
	}

	// Parse CA certificate if provided
	if len(caCertPEM) > 0 {
		caCert, err := parseCertificate(caCertPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
		}
		svc.caCert = caCert
		svc.caCertPool = x509.NewCertPool()
		svc.caCertPool.AppendCertsFromPEM(caCertPEM)
	}

	// Load existing identities from storage
	if err := svc.loadIdentities(); err != nil {
		logger.Warn("Failed to load identities from storage", logger.ErrField(err))
	}

	// Load CRL from storage
	if err := svc.loadCRL(); err != nil {
		logger.Warn("Failed to load CRL from storage", logger.ErrField(err))
	}

	// Start background tasks
	svc.wg.Add(2)
	go svc.expirationChecker()
	go svc.rotationChecker()

	logger.Info("Identity service initialized",
		logger.Int("cached_identities", len(svc.identityCache)),
		logger.Int("crl_entries", len(svc.crl.Entries)),
	)

	return svc, nil
}

// Stop gracefully stops the identity service
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.Info("Identity service stopped")
}

// RegisterVehicle registers a new vehicle identity (Task 4.2)
func (s *Service) RegisterVehicle(req *RegistrationRequest) (*VehicleIdentity, error) {
	// Check if already registered
	if _, exists := s.GetIdentity(req.VehicleID); exists {
		return nil, fmt.Errorf("vehicle already registered: %s", req.VehicleID.String())
	}

	// Verify request signature
	if err := req.VerifyRequestSignature(); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Parse and verify certificate (Task 4.3)
	cert, err := parseCertificate(req.Certificate)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate: %w", err)
	}

	// Verify certificate chain if CA is configured
	if s.caCertPool != nil {
		opts := x509.VerifyOptions{
			Roots:         s.caCertPool,
			CurrentTime:   time.Now(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		if _, err := cert.Verify(opts); err != nil {
			return nil, fmt.Errorf("certificate verification failed: %w", err)
		}
	}

	// Verify public key matches certificate
	certPubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal certificate public key: %w", err)
	}

	if !bytes.Equal(certPubKey, req.PublicKey) {
		return nil, fmt.Errorf("public key mismatch with certificate")
	}

	// Check if certificate is revoked (Task 4.5)
	if s.IsRevoked(cert.SerialNumber.String()) {
		return nil, fmt.Errorf("certificate has been revoked")
	}

	// Create identity
	certHash := crypto.Keccak256Hash(req.Certificate)
	var hash [32]byte
	copy(hash[:], certHash[:])
	identity := &VehicleIdentity{
		VehicleID:       req.VehicleID,
		PublicKey:       req.PublicKey,
		Certificate:     req.Certificate,
		CertificateHash: hash,
		FleetID:         req.FleetID,
		Status:          IdentityStatusActive,
		RegisteredAt:    time.Now().Unix(),
		ExpiresAt:       cert.NotAfter.Unix(),
		LastRotatedAt:   time.Now().Unix(),
		LastSeenAt:      time.Now().Unix(),
		CAName:          cert.Issuer.String(),
	}

	// Store identity
	if err := s.storeIdentity(identity); err != nil {
		return nil, fmt.Errorf("failed to store identity: %w", err)
	}

	// Update cache
	s.cacheMu.Lock()
	s.identityCache[identity.VehicleID] = identity
	s.cacheMu.Unlock()

	logger.Info("Vehicle registered successfully",
		logger.String("vehicle_id", identity.VehicleID.String()),
		logger.String("fleet_id", identity.FleetID),
		logger.Int64("expires_at", cert.NotAfter.Unix()),
	)

	return identity, nil
}

// BatchRegisterVehicles registers multiple vehicles in a batch (Task 4.10)
func (s *Service) BatchRegisterVehicles(req *BatchRegistrationRequest) ([]*VehicleIdentity, error) {
	if len(req.Requests) == 0 {
		return nil, fmt.Errorf("empty batch registration request")
	}

	// In a real implementation, verify CA signature on the batch
	// For now, we just process each request

	identities := make([]*VehicleIdentity, 0, len(req.Requests))
	for i, regReq := range req.Requests {
		// Set fleet ID from batch request
		regReq.FleetID = req.FleetID

		identity, err := s.RegisterVehicle(&regReq)
		if err != nil {
			logger.Warn("Failed to register vehicle in batch",
				logger.Int("index", i),
				logger.String("vehicle_id", regReq.VehicleID.String()),
				logger.ErrField(err),
			)
			continue
		}
		identities = append(identities, identity)
	}

	if len(identities) == 0 {
		return nil, fmt.Errorf("no vehicles were registered")
	}

	logger.Info("Batch registration completed",
		logger.String("fleet_id", req.FleetID),
		logger.Int("success", len(identities)),
		logger.Int("total", len(req.Requests)),
	)

	return identities, nil
}

// GetIdentity retrieves an identity by vehicle ID (Task 4.8)
func (s *Service) GetIdentity(vehicleID blockchain.Address) (*VehicleIdentity, bool) {
	// Check cache first
	s.cacheMu.RLock()
	if identity, exists := s.identityCache[vehicleID]; exists {
		s.cacheMu.RUnlock()
		return identity, true
	}
	s.cacheMu.RUnlock()

	// Load from storage
	identity, err := s.loadIdentityFromStorage(vehicleID)
	if err != nil {
		return nil, false
	}

	// Update cache
	s.cacheMu.Lock()
	s.identityCache[vehicleID] = identity
	s.cacheMu.Unlock()

	return identity, true
}

// QueryIdentity queries identity with online status (Task 4.8, 4.9)
func (s *Service) QueryIdentity(vehicleID blockchain.Address) *IdentityQueryResult {
	identity, found := s.GetIdentity(vehicleID)
	if !found {
		return &IdentityQueryResult{Found: false}
	}

	result := &IdentityQueryResult{
		Identity: identity,
		Found:    true,
	}

	// Get online status
	s.onlineMu.RLock()
	if status, exists := s.onlineStatus[vehicleID]; exists {
		result.Online = status
	}
	s.onlineMu.RUnlock()

	return result
}

// GetPublicKey retrieves the public key for a vehicle (Task 4.8)
func (s *Service) GetPublicKey(vehicleID blockchain.Address) ([]byte, error) {
	identity, exists := s.GetIdentity(vehicleID)
	if !exists {
		return nil, fmt.Errorf("identity not found: %s", vehicleID.String())
	}

	if !identity.IsActive() {
		return nil, fmt.Errorf("identity is not active: %s", identity.Status.String())
	}

	return identity.PublicKey, nil
}

// VerifyNodeAdmission checks if a node is allowed to join the network (Task 4.4)
func (s *Service) VerifyNodeAdmission(vehicleID blockchain.Address) error {
	identity, exists := s.GetIdentity(vehicleID)
	if !exists {
		return fmt.Errorf("identity not registered: %s", vehicleID.String())
	}

	if identity.Status == IdentityStatusRevoked {
		return fmt.Errorf("identity has been revoked")
	}

	if identity.IsExpired() {
		return fmt.Errorf("certificate has expired")
	}

	// Check CRL
	cert, err := identity.ParseCertificate()
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	if s.IsRevoked(cert.SerialNumber.String()) {
		return fmt.Errorf("certificate has been revoked")
	}

	return nil
}

// RevokeCertificate revokes a certificate (Task 4.5)
func (s *Service) RevokeCertificate(serialNumber string, reason string) error {
	s.crlMu.Lock()
	defer s.crlMu.Unlock()

	// Check if already revoked
	for _, entry := range s.crl.Entries {
		if entry.SerialNumber == serialNumber {
			return fmt.Errorf("certificate already revoked")
		}
	}

	// Add to CRL
	entry := CRLEntry{
		SerialNumber: serialNumber,
		RevokedAt:    time.Now().Unix(),
		Reason:       reason,
	}
	s.crl.Entries = append(s.crl.Entries, entry)
	s.crl.UpdatedAt = time.Now().Unix()
	s.crl.NextUpdate = time.Now().Add(24 * time.Hour).Unix()

	// Persist CRL
	if err := s.storeCRL(); err != nil {
		return fmt.Errorf("failed to store CRL: %w", err)
	}

	// Update identity status if found
	s.cacheMu.Lock()
	for _, identity := range s.identityCache {
		cert, err := identity.ParseCertificate()
		if err != nil {
			continue
		}
		if cert.SerialNumber.String() == serialNumber {
			identity.Status = IdentityStatusRevoked
			s.storeIdentity(identity)
			break
		}
	}
	s.cacheMu.Unlock()

	logger.Info("Certificate revoked",
		logger.String("serial_number", serialNumber),
		logger.String("reason", reason),
	)

	return nil
}

// IsRevoked checks if a certificate serial number is revoked (Task 4.5)
func (s *Service) IsRevoked(serialNumber string) bool {
	s.crlMu.RLock()
	defer s.crlMu.RUnlock()

	return s.crl.IsRevoked(serialNumber)
}

// RotateCertificate rotates a vehicle's certificate (Task 4.6)
func (s *Service) RotateCertificate(req *CertificateRotationRequest) error {
	identity, exists := s.GetIdentity(req.VehicleID)
	if !exists {
		return fmt.Errorf("identity not found")
	}

	// Verify old certificate hash matches
	if identity.CertificateHash != req.OldCertHash {
		return fmt.Errorf("old certificate hash mismatch")
	}

	// Parse new certificate
	newCert, err := parseCertificate(req.NewCertificate)
	if err != nil {
		return fmt.Errorf("invalid new certificate: %w", err)
	}

	// Verify certificate chain
	if s.caCertPool != nil {
		opts := x509.VerifyOptions{
			Roots:       s.caCertPool,
			CurrentTime: time.Now(),
		}
		if _, err := newCert.Verify(opts); err != nil {
			return fmt.Errorf("new certificate verification failed: %w", err)
		}
	}

	// Update identity
	identity.Certificate = req.NewCertificate
	newCertHash := crypto.Keccak256Hash(req.NewCertificate)
	copy(identity.CertificateHash[:], newCertHash[:])
	identity.ExpiresAt = newCert.NotAfter.Unix()
	identity.LastRotatedAt = time.Now().Unix()
	identity.Status = IdentityStatusActive

	// Store updated identity
	if err := s.storeIdentity(identity); err != nil {
		return fmt.Errorf("failed to store updated identity: %w", err)
	}

	// Update cache
	s.cacheMu.Lock()
	s.identityCache[identity.VehicleID] = identity
	s.cacheMu.Unlock()

	logger.Info("Certificate rotated successfully",
		logger.String("vehicle_id", identity.VehicleID.String()),
		logger.Int64("new_expires_at", newCert.NotAfter.Unix()),
	)

	return nil
}

// UpdateOnlineStatus updates the online status of a node (Task 4.9)
func (s *Service) UpdateOnlineStatus(vehicleID blockchain.Address, isOnline bool, networkAddr string) {
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()

	now := time.Now().Unix()

	status, exists := s.onlineStatus[vehicleID]
	if !exists {
		status = &OnlineStatus{
			VehicleID: vehicleID,
		}
		s.onlineStatus[vehicleID] = status
	}

	status.IsOnline = isOnline
	if isOnline {
		status.LastSeenAt = now
		status.NetworkAddr = networkAddr
	}

	// Update identity's last seen time
	if identity, found := s.GetIdentity(vehicleID); found {
		identity.LastSeenAt = now
	}
}

// GetOnlineStatus gets the online status of a node (Task 4.9)
func (s *Service) GetOnlineStatus(vehicleID blockchain.Address) (*OnlineStatus, bool) {
	s.onlineMu.RLock()
	defer s.onlineMu.RUnlock()

	status, exists := s.onlineStatus[vehicleID]
	return status, exists
}

// expirationChecker periodically checks for expired certificates (Task 4.7)
func (s *Service) expirationChecker() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkExpiredCertificates()
		}
	}
}

// checkExpiredCertificates checks and updates expired certificates
func (s *Service) checkExpiredCertificates() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	now := time.Now().Unix()
	expiredCount := 0

	for _, identity := range s.identityCache {
		if identity.Status == IdentityStatusActive && now > identity.ExpiresAt {
			identity.Status = IdentityStatusExpired
			if err := s.storeIdentity(identity); err != nil {
				logger.Warn("Failed to update expired identity",
					logger.String("vehicle_id", identity.VehicleID.String()),
					logger.ErrField(err),
				)
			} else {
				expiredCount++
				logger.Info("Certificate expired",
					logger.String("vehicle_id", identity.VehicleID.String()),
				)
			}
		}
	}

	if expiredCount > 0 {
		logger.Info("Expiration check completed", logger.Int("expired_count", expiredCount))
	}
}

// rotationChecker periodically checks for certificates needing rotation (Task 4.6)
func (s *Service) rotationChecker() {
	defer s.wg.Done()

	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkCertificatesForRotation()
		}
	}
}

// checkCertificatesForRotation checks for certificates needing rotation
func (s *Service) checkCertificatesForRotation() {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	rotationCount := 0

	for _, identity := range s.identityCache {
		if identity.IsActive() && identity.NeedsRotation(s.rotateDaysBefore) {
			rotationCount++
			logger.Info("Certificate needs rotation",
				logger.String("vehicle_id", identity.VehicleID.String()),
				logger.Int64("expires_at", identity.ExpiresAt),
			)
			// In a real implementation, this would trigger automatic rotation
		}
	}

	if rotationCount > 0 {
		logger.Info("Rotation check completed", logger.Int("needs_rotation", rotationCount))
	}
}

// Storage helpers

func (s *Service) storeIdentity(identity *VehicleIdentity) error {
	data, err := json.Marshal(identity)
	if err != nil {
		return err
	}

	key := append(storage.PrefixIdentity, identity.VehicleID[:]...)
	return s.storage.Put(key, data)
}

func (s *Service) loadIdentityFromStorage(vehicleID blockchain.Address) (*VehicleIdentity, error) {
	key := append(storage.PrefixIdentity, vehicleID[:]...)
	data, err := s.storage.Get(key)
	if err != nil {
		return nil, err
	}

	var identity VehicleIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, err
	}

	return &identity, nil
}

func (s *Service) loadIdentities() error {
	iter := s.storage.NewIterator(storage.PrefixIdentity)
	defer iter.Release()

	count := 0
	for iter.Next() {
		var identity VehicleIdentity
		if err := json.Unmarshal(iter.Value(), &identity); err != nil {
			logger.Warn("Failed to unmarshal identity", logger.ErrField(err))
			continue
		}

		s.identityCache[identity.VehicleID] = &identity
		count++
	}

	return iter.Error()
}

func (s *Service) storeCRL() error {
	data, err := json.Marshal(s.crl)
	if err != nil {
		return err
	}

	key := append(storage.PrefixMetadata, []byte("crl")...)
	return s.storage.Put(key, data)
}

func (s *Service) loadCRL() error {
	key := append(storage.PrefixMetadata, []byte("crl")...)
	data, err := s.storage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil // No CRL stored yet
		}
		return err
	}

	return json.Unmarshal(data, s.crl)
}

func parseCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}
