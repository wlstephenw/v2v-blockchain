package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newPlatoonCmd creates platoon management commands (Task 10.3)
func newPlatoonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platoon",
		Short: "Platoon management commands",
		Long:  `Manage vehicle platoons including creation, joining, leaving, and status queries.`,
	}

	cmd.AddCommand(newPlatoonCreateCmd())
	cmd.AddCommand(newPlatoonJoinCmd())
	cmd.AddCommand(newPlatoonLeaveCmd())
	cmd.AddCommand(newPlatoonDissolveCmd())
	cmd.AddCommand(newPlatoonListCmd())
	cmd.AddCommand(newPlatoonStatusCmd())

	return cmd
}

func newPlatoonCreateCmd() *cobra.Command {
	var (
		platoonID    string
		leaderID     string
		minGap       float64
		maxSize      int
		targetSpeed  float64
		validatorIDs []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new platoon",
		Long:  `Create a new vehicle platoon with specified parameters.`,
		Example: `  v2v-node platoon create --platoon-id platoon-001 --leader 0x1234... --min-gap 2.0 --max-size 8`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonCreate(platoonID, leaderID, minGap, maxSize, targetSpeed, validatorIDs)
		},
	}

	cmd.Flags().StringVar(&platoonID, "platoon-id", "", "Unique platoon ID (required)")
	cmd.Flags().StringVar(&leaderID, "leader", "", "Leader vehicle ID (required)")
	cmd.Flags().Float64Var(&minGap, "min-gap", 2.0, "Minimum safe gap (meters)")
	cmd.Flags().IntVar(&maxSize, "max-size", 8, "Maximum platoon size")
	cmd.Flags().Float64Var(&targetSpeed, "target-speed", 30.0, "Target speed (m/s)")
	cmd.Flags().StringArrayVar(&validatorIDs, "validators", nil, "Initial validator IDs")
	cmd.MarkFlagRequired("platoon-id")
	cmd.MarkFlagRequired("leader")

	return cmd
}

func runPlatoonCreate(platoonID, leaderID string, minGap float64, maxSize int, targetSpeed float64, validatorIDs []string) error {
	// Create creation request (simplified)
	req := map[string]interface{}{
		"action":       "create_platoon",
		"platoon_id":   platoonID,
		"leader_id":    leaderID,
		"min_gap":      minGap,
		"max_size":     maxSize,
		"target_speed": targetSpeed,
		"validators":   validatorIDs,
	}

	// Output the request for submission
	output := map[string]interface{}{
		"action":  "create_platoon",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}

func newPlatoonJoinCmd() *cobra.Command {
	var (
		platoonID string
		vehicleID string
	)

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a platoon",
		Long:  `Request to join an existing platoon.`,
		Example: `  v2v-node platoon join --platoon-id platoon-001 --vehicle-id 0x5678...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonJoin(platoonID, vehicleID)
		},
	}

	cmd.Flags().StringVar(&platoonID, "platoon-id", "", "Platoon ID to join (required)")
	cmd.Flags().StringVar(&vehicleID, "vehicle-id", "", "Vehicle ID joining (required)")
	cmd.MarkFlagRequired("platoon-id")
	cmd.MarkFlagRequired("vehicle-id")

	return cmd
}

func runPlatoonJoin(platoonID, vehicleID string) error {
	req := map[string]interface{}{
		"platoon_id": platoonID,
		"vehicle_id": vehicleID,
	}

	output := map[string]interface{}{
		"action":  "join_platoon",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}

func newPlatoonLeaveCmd() *cobra.Command {
	var (
		platoonID string
		vehicleID string
	)

	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave a platoon",
		Long:  `Request to leave a platoon.`,
		Example: `  v2v-node platoon leave --platoon-id platoon-001 --vehicle-id 0x5678...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonLeave(platoonID, vehicleID)
		},
	}

	cmd.Flags().StringVar(&platoonID, "platoon-id", "", "Platoon ID to leave (required)")
	cmd.Flags().StringVar(&vehicleID, "vehicle-id", "", "Vehicle ID leaving (required)")
	cmd.MarkFlagRequired("platoon-id")
	cmd.MarkFlagRequired("vehicle-id")

	return cmd
}

func runPlatoonLeave(platoonID, vehicleID string) error {
	req := map[string]interface{}{
		"platoon_id": platoonID,
		"vehicle_id": vehicleID,
	}

	output := map[string]interface{}{
		"action":  "leave_platoon",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}

func newPlatoonDissolveCmd() *cobra.Command {
	var (
		platoonID string
		reason    string
	)

	cmd := &cobra.Command{
		Use:   "dissolve",
		Short: "Dissolve a platoon",
		Long:  `Dissolve an existing platoon (leader only).`,
		Example: `  v2v-node platoon dissolve --platoon-id platoon-001 --reason "destination reached"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonDissolve(platoonID, reason)
		},
	}

	cmd.Flags().StringVar(&platoonID, "platoon-id", "", "Platoon ID to dissolve (required)")
	cmd.Flags().StringVar(&reason, "reason", "manual", "Reason for dissolution")
	cmd.MarkFlagRequired("platoon-id")

	return cmd
}

func runPlatoonDissolve(platoonID, reason string) error {
	req := map[string]interface{}{
		"platoon_id": platoonID,
		"reason":     reason,
	}

	output := map[string]interface{}{
		"action":  "dissolve_platoon",
		"request": req,
		"note":    "Submit this request via API: POST /api/v1/transactions",
	}

	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}

func newPlatoonListCmd() *cobra.Command {
	var (
		status string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List platoons",
		Long:  `List all platoons with optional status filter.`,
		Example: `  v2v-node platoon list --status active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonList(status)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (active|dissolved)")

	return cmd
}

func runPlatoonList(status string) error {
	fmt.Println("Listing platoons")
	if status != "" {
		fmt.Printf("Filter: status=%s\n", status)
	}
	fmt.Println("Query via API: GET /api/v1/platoons")
	return nil
}

func newPlatoonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [platoon-id]",
		Short: "Get platoon status",
		Long:  `Get detailed status of a specific platoon.`,
		Args:  cobra.ExactArgs(1),
		Example: `  v2v-node platoon status platoon-001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlatoonStatus(args[0])
		},
	}
}

func runPlatoonStatus(platoonID string) error {
	fmt.Printf("Getting status for platoon: %s\n", platoonID)
	fmt.Printf("Query via API: GET /api/v1/platoons/%s\n", platoonID)
	return nil
}
