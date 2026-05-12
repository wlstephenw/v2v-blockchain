package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// newIdentityCmd creates identity management commands (Task 10.2)
func newIdentityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Vehicle identity management commands",
		Long:  `Manage vehicle identities including registration, certificate rotation, and status checks.`,
	}

	cmd.AddCommand(newIdentityRegisterCmd())
	cmd.AddCommand(newIdentityRotateCmd())
	cmd.AddCommand(newIdentityStatusCmd())
	cmd.AddCommand(newIdentityListCmd())

	return cmd
}

func newIdentityRegisterCmd() *cobra.Command {
	var (
		vehicleID   string
		publicKey   string
		certificate string
		fleetID     string
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new vehicle identity",
		Long:  `Register a new vehicle with the blockchain network.`,
		Example: `  v2v-node identity register --vehicle-id 0x1234... --public-key 0xabcd... --cert cert.pem --fleet-id fleet-001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityRegister(vehicleID, publicKey, certificate, fleetID)
		},
	}

	cmd.Flags().StringVar(&vehicleID, "vehicle-id", "", "Vehicle address (required)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "Vehicle public key hex (required)")
	cmd.Flags().StringVar(&certificate, "cert", "", "Certificate file path (required)")
	cmd.Flags().StringVar(&fleetID, "fleet-id", "", "Fleet ID (required)")
	cmd.MarkFlagRequired("vehicle-id")
	cmd.MarkFlagRequired("public-key")
	cmd.MarkFlagRequired("cert")
	cmd.MarkFlagRequired("fleet-id")

	return cmd
}

func runIdentityRegister(vehicleID, publicKey, certificate, fleetID string) error {
	// Read certificate file
	_, err := os.ReadFile(certificate)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	// Decode public key
	_, err = hex.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Create registration request
	req := map[string]interface{}{
		"vehicle_id":   vehicleID,
		"public_key":   publicKey,
		"certificate":  "<certificate data>",
		"fleet_id":     fleetID,
	}

	// Output the request for submission
	output := map[string]interface{}{
		"action":  "register_identity",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	logger.Info("Identity registration prepared",
		logger.String("vehicle_id", vehicleID),
		logger.String("fleet_id", fleetID),
	)

	return nil
}

func newIdentityRotateCmd() *cobra.Command {
	var (
		vehicleID      string
		newCertificate string
	)

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate vehicle certificate",
		Long:  `Rotate the certificate for an existing vehicle identity.`,
		Example: `  v2v-node identity rotate --vehicle-id 0x1234... --new-cert new-cert.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityRotate(vehicleID, newCertificate)
		},
	}

	cmd.Flags().StringVar(&vehicleID, "vehicle-id", "", "Vehicle address (required)")
	cmd.Flags().StringVar(&newCertificate, "new-cert", "", "New certificate file path (required)")
	cmd.MarkFlagRequired("vehicle-id")
	cmd.MarkFlagRequired("new-cert")

	return cmd
}

func runIdentityRotate(vehicleID, newCertificate string) error {
	// Read new certificate file
	_, err := os.ReadFile(newCertificate)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	// Create rotation request
	req := map[string]interface{}{
		"vehicle_id":      vehicleID,
		"new_certificate": "<certificate data>",
	}

	// Output the request for submission
	output := map[string]interface{}{
		"action":  "rotate_certificate",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	logger.Info("Certificate rotation prepared",
		logger.String("vehicle_id", vehicleID),
	)

	return nil
}

func newIdentityStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [vehicle-id]",
		Short: "Check vehicle identity status",
		Long:  `Check the status of a vehicle identity.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityStatus(args[0])
		},
	}
}

func runIdentityStatus(vehicleID string) error {
	fmt.Printf("Checking identity status for: %s\n", vehicleID)
	fmt.Printf("Query via API: GET /api/v1/identities/%s\n", vehicleID)
	return nil
}

func newIdentityListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered identities",
		Long:  `List all registered vehicle identities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityList()
		},
	}
}

func runIdentityList() error {
	fmt.Println("Listing registered identities")
	fmt.Println("Query via API: GET /api/v1/identities")
	return nil
}

// Helper function to parse address
func parseAddress(s string) [20]byte {
	var addr [20]byte
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	if len(s) == 40 {
		data, _ := hex.DecodeString(s)
		copy(addr[:], data)
	}
	return addr
}
