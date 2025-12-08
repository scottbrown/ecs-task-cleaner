package main

import (
	"os"

	ecstaskcleaner "github.com/scottbrown/ecs-task-cleaner"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		profile  string
		region   string
		keep     int
		dryRun   bool
		batchSize int
	)

	cmd := &cobra.Command{
		Use:   "ecs-task-cleaner [family]",
		Short: "Clean up old ECS task definition revisions",
		Long: `A CLI tool to deregister and delete old ECS task definition revisions.

This tool will list all task definitions for a given family, keep the most
recent N revisions (default: 5), and deregister then delete the rest.

Example:
  ecs-task-cleaner accounts-api-STAGING --profile staging --keep 10
  ecs-task-cleaner accounts-api-STAGING --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			family := args[0]

			cfg := ecstaskcleaner.Config{
				Family:    family,
				Profile:   profile,
				Region:    region,
				Keep:      keep,
				DryRun:    dryRun,
				BatchSize: batchSize,
			}

			cleaner, err := ecstaskcleaner.New(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			return cleaner.Run(cmd.Context(), cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS profile to use")
	cmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (defaults to profile/environment default)")
	cmd.Flags().IntVarP(&keep, "keep", "k", 5, "Number of most recent revisions to keep")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be deleted without making changes")
	cmd.Flags().IntVarP(&batchSize, "batch-size", "b", 10, "Number of task definitions to delete per API call (max 10)")

	return cmd
}
