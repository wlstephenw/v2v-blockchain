package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Get node status",
		Long:  `Get the current status of the V2V blockchain node.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	fmt.Println("V2V Blockchain Node Status")
	fmt.Println("==========================")
	fmt.Println()
	fmt.Println("API Endpoints:")
	fmt.Println("  GET  /health                    - Health check")
	fmt.Println("  GET  /ready                     - Readiness check")
	fmt.Println("  GET  /api/v1/node/status        - Node status")
	fmt.Println("  GET  /api/v1/node/peers         - Connected peers")
	fmt.Println("  GET  /api/v1/node/stats         - Node statistics")
	fmt.Println()
	fmt.Println("Block APIs:")
	fmt.Println("  GET  /api/v1/blocks/latest      - Latest block")
	fmt.Println("  GET  /api/v1/blocks/{height}    - Block by height")
	fmt.Println("  GET  /api/v1/blocks/hash/{hash} - Block by hash")
	fmt.Println("  GET  /api/v1/blocks             - List blocks")
	fmt.Println()
	fmt.Println("Transaction APIs:")
	fmt.Println("  POST /api/v1/transactions       - Submit transaction")
	fmt.Println("  GET  /api/v1/transactions/{hash} - Get transaction")
	fmt.Println("  GET  /api/v1/transactions/pending - Pending transactions")
	fmt.Println()
	fmt.Println("Platoon APIs:")
	fmt.Println("  GET  /api/v1/platoons           - List platoons")
	fmt.Println("  GET  /api/v1/platoons/{id}      - Get platoon")
	fmt.Println("  GET  /api/v1/platoons/{id}/members - Get members")
	fmt.Println("  GET  /api/v1/platoons/{id}/history - Get history")
	fmt.Println()
	fmt.Println("Identity APIs:")
	fmt.Println("  GET  /api/v1/identities         - List identities")
	fmt.Println("  GET  /api/v1/identities/{id}    - Get identity")
	fmt.Println()
	fmt.Println("WebSocket:")
	fmt.Println("  WS   /ws                        - Real-time updates")
	fmt.Println()
	return nil
}
