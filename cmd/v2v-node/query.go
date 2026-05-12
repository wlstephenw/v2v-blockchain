package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newQueryCmd creates query commands (Task 10.4)
func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query blockchain data",
		Long:  `Query blocks, transactions, identities, and platoons from the blockchain.`,
	}

	cmd.AddCommand(newQueryBlockCmd())
	cmd.AddCommand(newQueryTxCmd())
	cmd.AddCommand(newQueryIdentityCmd())
	cmd.AddCommand(newQueryPlatoonCmd())

	return cmd
}

func newQueryBlockCmd() *cobra.Command {
	var (
		height uint64
		hash   string
		latest bool
	)

	cmd := &cobra.Command{
		Use:   "block",
		Short: "Query block information",
		Long:  `Query block by height, hash, or get the latest block.`,
		Example: `  v2v-node query block --latest
  v2v-node query block --height 100
  v2v-node query block --hash 0xabcd...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueryBlock(height, hash, latest)
		},
	}

	cmd.Flags().Uint64Var(&height, "height", 0, "Block height")
	cmd.Flags().StringVar(&hash, "hash", "", "Block hash")
	cmd.Flags().BoolVar(&latest, "latest", false, "Get latest block")

	return cmd
}

func runQueryBlock(height uint64, hash string, latest bool) error {
	if latest {
		fmt.Println("Querying latest block")
		fmt.Println("API: GET /api/v1/blocks/latest")
	} else if hash != "" {
		fmt.Printf("Querying block by hash: %s\n", hash)
		fmt.Printf("API: GET /api/v1/blocks/hash/%s\n", hash)
	} else if height > 0 {
		fmt.Printf("Querying block at height: %d\n", height)
		fmt.Printf("API: GET /api/v1/blocks/%d\n", height)
	} else {
		fmt.Println("Querying recent blocks")
		fmt.Println("API: GET /api/v1/blocks")
	}
	return nil
}

func newQueryTxCmd() *cobra.Command {
	var (
		hash     string
		pending  bool
	)

	cmd := &cobra.Command{
		Use:   "tx",
		Short: "Query transaction",
		Long:  `Query transaction by hash or list pending transactions.`,
		Example: `  v2v-node query tx --hash 0xabcd...
  v2v-node query tx --pending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueryTx(hash, pending)
		},
	}

	cmd.Flags().StringVar(&hash, "hash", "", "Transaction hash")
	cmd.Flags().BoolVar(&pending, "pending", false, "List pending transactions")

	return cmd
}

func runQueryTx(hash string, pending bool) error {
	if pending {
		fmt.Println("Querying pending transactions")
		fmt.Println("API: GET /api/v1/transactions/pending")
	} else if hash != "" {
		fmt.Printf("Querying transaction: %s\n", hash)
		fmt.Printf("API: GET /api/v1/transactions/%s\n", hash)
	} else {
		fmt.Println("Please specify --hash or --pending")
	}
	return nil
}

func newQueryIdentityCmd() *cobra.Command {
	var (
		vehicleID string
		list      bool
	)

	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Query identity information",
		Long:  `Query vehicle identity by ID or list all identities.`,
		Example: `  v2v-node query identity --vehicle-id 0x1234...
  v2v-node query identity --list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueryIdentity(vehicleID, list)
		},
	}

	cmd.Flags().StringVar(&vehicleID, "vehicle-id", "", "Vehicle ID")
	cmd.Flags().BoolVar(&list, "list", false, "List all identities")

	return cmd
}

func runQueryIdentity(vehicleID string, list bool) error {
	if list {
		fmt.Println("Listing all identities")
		fmt.Println("API: GET /api/v1/identities")
	} else if vehicleID != "" {
		fmt.Printf("Querying identity: %s\n", vehicleID)
		fmt.Printf("API: GET /api/v1/identities/%s\n", vehicleID)
	} else {
		fmt.Println("Please specify --vehicle-id or --list")
	}
	return nil
}

func newQueryPlatoonCmd() *cobra.Command {
	var (
		platoonID string
		list      bool
		members   bool
		history   bool
	)

	cmd := &cobra.Command{
		Use:   "platoon",
		Short: "Query platoon information",
		Long:  `Query platoon by ID, list platoons, or get members/history.`,
		Example: `  v2v-node query platoon --list
  v2v-node query platoon --platoon-id platoon-001
  v2v-node query platoon --platoon-id platoon-001 --members
  v2v-node query platoon --platoon-id platoon-001 --history`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueryPlatoon(platoonID, list, members, history)
		},
	}

	cmd.Flags().StringVar(&platoonID, "platoon-id", "", "Platoon ID")
	cmd.Flags().BoolVar(&list, "list", false, "List all platoons")
	cmd.Flags().BoolVar(&members, "members", false, "Get platoon members")
	cmd.Flags().BoolVar(&history, "history", false, "Get platoon history")

	return cmd
}

func runQueryPlatoon(platoonID string, list, members, history bool) error {
	if list {
		fmt.Println("Listing all platoons")
		fmt.Println("API: GET /api/v1/platoons")
	} else if platoonID != "" {
		if members {
			fmt.Printf("Getting members for platoon: %s\n", platoonID)
			fmt.Printf("API: GET /api/v1/platoons/%s/members\n", platoonID)
		} else if history {
			fmt.Printf("Getting history for platoon: %s\n", platoonID)
			fmt.Printf("API: GET /api/v1/platoons/%s/history\n", platoonID)
		} else {
			fmt.Printf("Querying platoon: %s\n", platoonID)
			fmt.Printf("API: GET /api/v1/platoons/%s\n", platoonID)
		}
	} else {
		fmt.Println("Please specify --platoon-id or --list")
	}
	return nil
}
