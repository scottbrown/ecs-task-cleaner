package ecstaskcleaner

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type mockECSClient struct {
	taskDefinitions     []string
	deregisteredArns    []string
	deletedArns         []string
	listError           error
	deregisterError     error
	deleteError         error
	deleteFailures      []types.Failure
	listCallCount       int
	deregisterCallCount int
	deleteCallCount     int
}

func (m *mockECSClient) ListTaskDefinitions(ctx context.Context, params *ecs.ListTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
	m.listCallCount++
	if m.listError != nil {
		return nil, m.listError
	}

	return &ecs.ListTaskDefinitionsOutput{
		TaskDefinitionArns: m.taskDefinitions,
	}, nil
}

func (m *mockECSClient) DeregisterTaskDefinition(ctx context.Context, params *ecs.DeregisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DeregisterTaskDefinitionOutput, error) {
	m.deregisterCallCount++
	if m.deregisterError != nil {
		return nil, m.deregisterError
	}

	m.deregisteredArns = append(m.deregisteredArns, aws.ToString(params.TaskDefinition))
	return &ecs.DeregisterTaskDefinitionOutput{}, nil
}

func (m *mockECSClient) DeleteTaskDefinitions(ctx context.Context, params *ecs.DeleteTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
	m.deleteCallCount++
	if m.deleteError != nil {
		return nil, m.deleteError
	}

	m.deletedArns = append(m.deletedArns, params.TaskDefinitions...)
	return &ecs.DeleteTaskDefinitionsOutput{
		Failures: m.deleteFailures,
	}, nil
}

func TestRun_NothingToDelete(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:3",
		},
	}

	cfg := Config{
		Family:    "test",
		Keep:      5,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Nothing to delete") {
		t.Errorf("expected 'Nothing to delete' message, got: %s", output)
	}

	if len(mock.deregisteredArns) > 0 {
		t.Errorf("expected no deregistrations, got %d", len(mock.deregisteredArns))
	}
}

func TestRun_DeletesOldRevisions(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:3",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:4",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:5",
		},
	}

	cfg := Config{
		Family:    "test",
		Keep:      2,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDeleted := 3
	if len(mock.deregisteredArns) != expectedDeleted {
		t.Errorf("expected %d deregistrations, got %d", expectedDeleted, len(mock.deregisteredArns))
	}

	if len(mock.deletedArns) != expectedDeleted {
		t.Errorf("expected %d deletions, got %d", expectedDeleted, len(mock.deletedArns))
	}

	// Verify the correct ARNs were deleted (oldest ones)
	for i, arn := range mock.deletedArns {
		expected := mock.taskDefinitions[i]
		if arn != expected {
			t.Errorf("expected deletion of %s, got %s", expected, arn)
		}
	}
}

func TestRun_DryRunDoesNotDelete(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:3",
		},
	}

	cfg := Config{
		Family:    "test",
		Keep:      1,
		DryRun:    true,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[DRY RUN]") {
		t.Errorf("expected dry run message, got: %s", output)
	}

	if len(mock.deregisteredArns) > 0 {
		t.Errorf("dry run should not deregister, got %d", len(mock.deregisteredArns))
	}

	if len(mock.deletedArns) > 0 {
		t.Errorf("dry run should not delete, got %d", len(mock.deletedArns))
	}
}

func TestRun_ListError(t *testing.T) {
	mock := &mockECSClient{
		listError: errors.New("access denied"),
	}

	cfg := Config{
		Family:    "test",
		Keep:      5,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "listing task definitions") {
		t.Errorf("expected list error, got: %v", err)
	}
}

func TestRun_DeregisterError(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
		},
		deregisterError: errors.New("throttling"),
	}

	cfg := Config{
		Family:    "test",
		Keep:      1,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "deregistering") {
		t.Errorf("expected deregister error, got: %v", err)
	}
}

func TestRun_DeleteError(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
		},
		deleteError: errors.New("service unavailable"),
	}

	cfg := Config{
		Family:    "test",
		Keep:      1,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "deleting batch") {
		t.Errorf("expected delete error, got: %v", err)
	}
}

func TestRun_BatchesCorrectly(t *testing.T) {
	// Create 25 task definitions
	var taskDefs []string
	for i := 1; i <= 25; i++ {
		taskDefs = append(taskDefs, "arn:aws:ecs:us-west-2:123456789:task-definition/test:"+string(rune('0'+i%10)))
	}

	mock := &mockECSClient{
		taskDefinitions: taskDefs,
	}

	cfg := Config{
		Family:    "test",
		Keep:      5,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should delete 20 items (25 - 5 to keep)
	expectedDeleted := 20
	if len(mock.deletedArns) != expectedDeleted {
		t.Errorf("expected %d deletions, got %d", expectedDeleted, len(mock.deletedArns))
	}

	// With batch size 10, should have 2 delete calls
	expectedBatches := 2
	if mock.deleteCallCount != expectedBatches {
		t.Errorf("expected %d delete batches, got %d", expectedBatches, mock.deleteCallCount)
	}
}

func TestRun_HandlesPartialFailures(t *testing.T) {
	mock := &mockECSClient{
		taskDefinitions: []string{
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:1",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:2",
			"arn:aws:ecs:us-west-2:123456789:task-definition/test:3",
		},
		deleteFailures: []types.Failure{
			{
				Arn:    aws.String("arn:aws:ecs:us-west-2:123456789:task-definition/test:1"),
				Reason: aws.String("resource in use"),
			},
		},
	}

	cfg := Config{
		Family:    "test",
		Keep:      1,
		BatchSize: 10,
	}

	cleaner := NewWithClient(mock, cfg)
	var buf bytes.Buffer
	err := cleaner.Run(context.Background(), &buf)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "WARNING") {
		t.Errorf("expected warning about failure, got: %s", output)
	}

	if !strings.Contains(output, "Successfully deleted 1 task definitions") {
		t.Errorf("expected correct count of successful deletions, got: %s", output)
	}
}

func TestNewWithClient_BatchSizeValidation(t *testing.T) {
	tests := []struct {
		name          string
		inputBatch    int
		expectedBatch int
	}{
		{"zero becomes 10", 0, 10},
		{"negative becomes 10", -5, 10},
		{"over 10 becomes 10", 15, 10},
		{"valid value stays", 5, 5},
		{"max value stays", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockECSClient{}
			cfg := Config{BatchSize: tt.inputBatch}
			cleaner := NewWithClient(mock, cfg)

			if cleaner.config.BatchSize != tt.expectedBatch {
				t.Errorf("expected batch size %d, got %d", tt.expectedBatch, cleaner.config.BatchSize)
			}
		})
	}
}
