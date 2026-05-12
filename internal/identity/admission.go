package identity

import (
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// AdmissionController implements libp2p connection gating for node admission control (Task 4.4)
type AdmissionController struct {
	identityService *Service
	localHost       host.Host
}

// NewAdmissionController creates a new admission controller
func NewAdmissionController(idService *Service, h host.Host) *AdmissionController {
	return &AdmissionController{
		identityService: idService,
		localHost:       h,
	}
}

// InterceptPeerDial is called when dialing a peer
func (ac *AdmissionController) InterceptPeerDial(p peer.ID) (allow bool) {
	// Allow outbound dials - we'll verify after connection
	return true
}

// InterceptAddrDial is called when dialing a specific address
func (ac *AdmissionController) InterceptAddrDial(id peer.ID, addr multiaddr.Multiaddr) (allow bool) {
	return true
}

// InterceptAccept is called when accepting an incoming connection
func (ac *AdmissionController) InterceptAccept(cm network.ConnMultiaddrs) (allow bool) {
	// Accept the connection first, we'll verify identity at the protocol level
	return true
}

// InterceptSecured is called after security handshake but before stream multiplexing
func (ac *AdmissionController) InterceptSecured(direction network.Direction, id peer.ID, cm network.ConnMultiaddrs) (allow bool) {
	// Check if peer is in our identity registry
	// For now, we allow all and do application-level verification
	return true
}

// InterceptUpgraded is called after stream multiplexing
func (ac *AdmissionController) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// ConnectionGater implements advanced connection filtering
type ConnectionGater struct {
	identityService *Service
	blockedPeers    map[peer.ID]bool
}

// NewConnectionGater creates a new connection gater
func NewConnectionGater(idService *Service) *ConnectionGater {
	return &ConnectionGater{
		identityService: idService,
		blockedPeers:    make(map[peer.ID]bool),
	}
}

// InterceptPeerDial prevents dialing certain peers
func (cg *ConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	return !cg.blockedPeers[p]
}

// InterceptAddrDial prevents dialing specific addresses
func (cg *ConnectionGater) InterceptAddrDial(id peer.ID, addr multiaddr.Multiaddr) (allow bool) {
	return !cg.blockedPeers[id]
}

// InterceptAccept prevents accepting connections from certain peers
func (cg *ConnectionGater) InterceptAccept(cm network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptSecured is called after the security handshake
func (cg *ConnectionGater) InterceptSecured(direction network.Direction, id peer.ID, cm network.ConnMultiaddrs) (allow bool) {
	return !cg.blockedPeers[id]
}

// InterceptUpgraded is called after the connection has been upgraded
func (cg *ConnectionGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// BlockPeer blocks a peer from connecting
func (cg *ConnectionGater) BlockPeer(id peer.ID) {
	cg.blockedPeers[id] = true
	logger.Info("Peer blocked", logger.String("peer_id", id.String()))
}

// UnblockPeer unblocks a previously blocked peer
func (cg *ConnectionGater) UnblockPeer(id peer.ID) {
	delete(cg.blockedPeers, id)
	logger.Info("Peer unblocked", logger.String("peer_id", id.String()))
}

// ProtocolInterceptor intercepts protocol-level authentication
type ProtocolInterceptor struct {
	identityService *Service
}

// NewProtocolInterceptor creates a new protocol interceptor
func NewProtocolInterceptor(idService *Service) *ProtocolInterceptor {
	return &ProtocolInterceptor{
		identityService: idService,
	}
}

// AuthenticatePeer authenticates a peer using their vehicle identity
func (pi *ProtocolInterceptor) AuthenticatePeer(vehicleID blockchain.Address) error {
	return pi.identityService.VerifyNodeAdmission(vehicleID)
}

// IsAllowed checks if a peer is allowed to participate in the network
func (pi *ProtocolInterceptor) IsAllowed(vehicleID blockchain.Address) bool {
	return pi.identityService.VerifyNodeAdmission(vehicleID) == nil
}

// Ensure interfaces are implemented
var _ connmgr.ConnectionGater = (*ConnectionGater)(nil)
