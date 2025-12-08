package ecstaskcleaner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// Config holds the configuration for the cleaner.
type Config struct {
	Family    string
	Profile   string
	Region    string
	Keep      int
	DryRun    bool
	BatchSize int
}

// Cleaner handles the deletion of old ECS task definitions.
type Cleaner struct {
	client ECSClient
	config Config
}

// New creates a new Cleaner with the given configuration.
func New(ctx context.Context, cfg Config) (*Cleaner, error) {
	if cfg.BatchSize < 1 || cfg.BatchSize > 10 {
		cfg.BatchSize = 10
	}

	awsCfg, err := loadAWSConfig(ctx, cfg.Profile, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := ecs.NewFromConfig(awsCfg)

	return &Cleaner{
		client: client,
		config: cfg,
	}, nil
}

// NewWithClient creates a new Cleaner with a provided ECS client (useful for testing).
func NewWithClient(client ECSClient, cfg Config) *Cleaner {
	if cfg.BatchSize < 1 || cfg.BatchSize > 10 {
		cfg.BatchSize = 10
	}

	return &Cleaner{
		client: client,
		config: cfg,
	}
}

func loadAWSConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

// Run executes the cleaning process.
func (c *Cleaner) Run(ctx context.Context, out io.Writer) error {
	fmt.Fprintf(out, "Listing task definitions for family: %s\n", c.config.Family)

	arns, err := c.listAllTaskDefinitions(ctx)
	if err != nil {
		return fmt.Errorf("listing task definitions: %w", err)
	}

	total := len(arns)
	fmt.Fprintf(out, "Found %d task definition revisions\n", total)

	if total <= c.config.Keep {
		fmt.Fprintf(out, "Nothing to delete (keeping %d, found %d)\n", c.config.Keep, total)
		return nil
	}

	toDelete := arns[:total-c.config.Keep]
	fmt.Fprintf(out, "Will delete %d revisions (keeping latest %d)\n", len(toDelete), c.config.Keep)

	if c.config.DryRun {
		fmt.Fprintln(out, "\n[DRY RUN] Would delete the following task definitions:")
		for _, arn := range toDelete {
			fmt.Fprintf(out, "  %s\n", arn)
		}
		return nil
	}

	return c.deleteTaskDefinitions(ctx, out, toDelete)
}

func (c *Cleaner) listAllTaskDefinitions(ctx context.Context) ([]string, error) {
	var allArns []string
	var nextToken *string

	for {
		input := &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String(c.config.Family),
			Sort:         types.SortOrderAsc,
			NextToken:    nextToken,
		}

		output, err := c.client.ListTaskDefinitions(ctx, input)
		if err != nil {
			return nil, err
		}

		allArns = append(allArns, output.TaskDefinitionArns...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return allArns, nil
}

func (c *Cleaner) deleteTaskDefinitions(ctx context.Context, out io.Writer, arns []string) error {
	total := len(arns)
	deleted := 0

	for i := 0; i < total; i += c.config.BatchSize {
		end := i + c.config.BatchSize
		if end > total {
			end = total
		}

		batch := arns[i:end]

		// First deregister the task definitions
		for _, arn := range batch {
			fmt.Fprintf(out, "Deregistering: %s\n", arn)
			_, err := c.client.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: aws.String(arn),
			})
			if err != nil {
				return fmt.Errorf("deregistering %s: %w", arn, err)
			}
		}

		// Then delete them in a batch
		fmt.Fprintf(out, "Deleting batch of %d task definitions...\n", len(batch))
		output, err := c.client.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
			TaskDefinitions: batch,
		})
		if err != nil {
			return fmt.Errorf("deleting batch: %w", err)
		}

		// Check for failures
		if len(output.Failures) > 0 {
			for _, failure := range output.Failures {
				fmt.Fprintf(out, "  WARNING: Failed to delete %s: %s\n",
					aws.ToString(failure.Arn), aws.ToString(failure.Reason))
			}
		}

		deleted += len(batch) - len(output.Failures)
		fmt.Fprintf(out, "Progress: %d/%d deleted\n\n", deleted, total)

		// Rate limiting: pause between batches to avoid throttling
		if end < total {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Fprintf(out, "Successfully deleted %d task definitions\n", deleted)
	return nil
}
