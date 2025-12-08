package ecstaskcleaner

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ECSClient defines the interface for ECS operations needed by the cleaner.
// This allows for easy testing with mock implementations.
type ECSClient interface {
	ListTaskDefinitions(ctx context.Context, params *ecs.ListTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
	DeregisterTaskDefinition(ctx context.Context, params *ecs.DeregisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DeregisterTaskDefinitionOutput, error)
	DeleteTaskDefinitions(ctx context.Context, params *ecs.DeleteTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error)
}
