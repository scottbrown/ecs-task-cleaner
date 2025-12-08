# ECS Task Cleaner

A CLI tool to bulk delete old ECS task definition revisions from AWS.

## Problem

AWS ECS task definitions accumulate over time as new revisions are deployed. The AWS Console requires clicking through each revision individually, which becomes tedious when you have thousands of revisions to clean up.

## Solution

This tool lists all task definitions for a given family, keeps the most recent N revisions (default: 5), and deregisters then deletes the rest in batches.

## Installation

```bash
# Install using go install
go install github.com/scottbrown/ecs-task-cleaner/cmd/ecs-task-cleaner@latest

# Or build from source
task build
```

## Usage

```bash
# Basic usage - delete old revisions keeping the latest 5
ecs-task-cleaner my-task-family --profile my-aws-profile

# Keep more revisions
ecs-task-cleaner my-task-family --profile staging --keep 10

# Dry run - see what would be deleted without making changes
ecs-task-cleaner my-task-family --profile staging --dry-run

# Specify region explicitly
ecs-task-cleaner my-task-family --profile staging --region us-west-2
```

## Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--profile` | `-p` | AWS profile to use | (environment default) |
| `--region` | `-r` | AWS region | (profile default) |
| `--keep` | `-k` | Number of most recent revisions to keep | 5 |
| `--dry-run` | `-n` | Show what would be deleted without making changes | false |
| `--batch-size` | `-b` | Number of task definitions to delete per API call (max 10) | 10 |

## Example

```bash
$ ecs-task-cleaner api-staging --profile staging --dry-run

Listing task definitions for family: api-staging
Found 3590 task definition revisions
Will delete 3585 revisions (keeping latest 5)

[DRY RUN] Would delete the following task definitions:
  arn:aws:ecs:us-west-2:123456789:task-definition/api-staging:1
  arn:aws:ecs:us-west-2:123456789:task-definition/api-staging:2
  ...
```

## Development

```bash
# Run tests
task test

# Run tests with coverage
task coverage

# Build
task build

# Format code
task fmt

# Vet
task vet
```

## How It Works

1. Lists all task definitions for the specified family (sorted ascending by revision)
2. Calculates which revisions to delete (all except the most recent N)
3. For each batch:
   - Deregisters each task definition (marks as INACTIVE)
   - Calls DeleteTaskDefinitions API (up to 10 per call)
   - Pauses 500ms between batches to avoid rate limiting

## Requirements

- Go 1.25+
- AWS credentials configured (via profile, environment variables, or IAM role)
- IAM permissions:
  - `ecs:ListTaskDefinitions`
  - `ecs:DeregisterTaskDefinition`
  - `ecs:DeleteTaskDefinitions`
