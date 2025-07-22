# fargate-td

A Go CLI tool for managing AWS Fargate task definitions using a hierarchical overlay/template system. It enables environment-specific configurations through YAML overlays and Go templates.

## Features

- **Hierarchical Configuration**: Organize task definitions using overlay directories for different environments and applications
- **Template Processing**: Dynamic container definitions using Go templates with variable substitution
- **AWS Integration**: Deploy to ECS services and CloudWatch Events (cron jobs)
- **Deployment Monitoring**: Track deployment progress in real-time
- **Diff Visualization**: Preview changes with color-coded diffs before deployment
- **Variable Override**: Command-line variables can override configuration values

## Installation

### Using go install (Recommended)

```bash
go install github.com/kazz187/fargate-td/cmd/fargate-td@latest
```

### From Source

```bash
git clone https://github.com/kazz187/fargate-td.git
cd fargate-td
go build -o fargate-td ./cmd/fargate-td
```

### Using Make

```bash
# Local development build
make build

# Production build (optimized)
make build-prod

# Cross-platform builds
make cross-build

# Create release packages
TAG=v1.0.0 make package
```

## Prerequisites

- Go 1.23.0 or higher
- AWS credentials configured via standard AWS credential chain
- AWS CLI or SDK access to ECS and CloudWatch Events

## Project Structure

fargate-td expects a specific directory structure for overlay configuration:

```
project-root/
├── config.yml                    # Deploy configuration
├── tasks/                        # Task definition overlays
│   ├── base.yml                  # Base task definition
│   └── app1/                     # App-specific directory
│       ├── base.yml              # App base configuration
│       ├── variables.yml         # App-level variables
│       └── development/          # Environment-specific
│           ├── base.yml          # Environment overrides
│           ├── variables.yml     # Environment variables
│           └── web.yml           # Specific task definition
└── containers/                   # Container definition templates
    └── app1/                     # Container name
        ├── base.yml              # Base container config
        ├── variables.yml         # Container variables
        ├── container.yml.tpl     # Container template
        └── development/          # Environment-specific
            ├── variables.yml     # Environment variables
            └── container.yml.tpl # Environment template
```

## Configuration

### Deploy Configuration (`config.yml`)

Maps task definitions to AWS services and cron jobs:

```yaml
clusters:
  - name: "production-cluster"
    services:
      - name: "web-service"
        task: "app1"
      - name: "api-service"
        task: "app2"
    cronJobs:
      - name: "daily-backup"
        task: "backup-task"
        cron: "0 2 * * *"
```

### Task Definition Overlays

Task definitions are built by merging YAML files from the hierarchy:

**Base Task (`tasks/base.yml`):**
```yaml
family: "default-family"
cpu: "256"
memory: "512"
networkMode: "awsvpc"
requiresCompatibilities:
  - "FARGATE"
executionRoleArn: "arn:aws:iam::123456789012:role/ecsTaskExecutionRole"
```

**App Override (`tasks/app1/base.yml`):**
```yaml
family: "app1-family"
cpu: "512"
```

**Environment Override (`tasks/app1/development/base.yml`):**
```yaml
cpu: "1024"
memory: "2048"
```

### Container Templates

Container definitions use Go templates with variable substitution:

**Template (`containers/app1/container.yml.tpl`):**
```yaml
name: "app1-container"
image: "my-app:{{.Version}}"
portMappings:
  - containerPort: {{.Port}}
    protocol: "tcp"
environment:
  - name: "ENV"
    value: "{{.Environment}}"
  - name: "DEBUG"
    value: "{{.Debug}}"
```

**Variables (`containers/app1/variables.yml`):**
```yaml
Version: "latest"
Port: 8080
Environment: "production"
Debug: "false"
```

## Commands

### variables

Display variables for a given path after overlay processing.

```bash
fargate-td variables -p app1/development
```

**Options:**
- `-p, --path` (required): Target path (e.g., `app1/development`)
- `-r, --root_path`: Project root path (default: current directory)
- `-v, --var`: Variables in key=value format (e.g., `-v"Version=0.0.1"`)
- `-d, --debug`: Enable debug logging

### generate

Generate complete task definitions by merging overlays and processing templates.

```bash
fargate-td generate -p app1/development -t web -v"Version=0.0.1"
```

**Options:**
- `-p, --path` (required): Target path
- `-t, --task` (required): Task name (cannot contain "/")
- `-r, --root_path`: Project root path
- `-v, --var`: Variables in key=value format
- `-d, --debug`: Enable debug logging

### deploy

Deploy task definitions to AWS ECS services and CloudWatch Events.

```bash
# Full deployment
fargate-td deploy -p app1/development -t web -v"Version=0.0.1"

# Task definition only (skip service updates)
fargate-td deploy -p app1/development -t web -v"Version=0.0.1" --td-only
```

**Options:**
- `-p, --path` (required): Target path
- `-t, --task` (required): Task name
- `-r, --root_path`: Project root path
- `-v, --var`: Variables in key=value format
- `--td-only`: Deploy task definition only (skip service/cron updates)
- `-d, --debug`: Enable debug logging

### watch

Monitor ECS service deployment status.

```bash
fargate-td watch -p app1/development -t web
```

**Options:**
- `-p, --path` (required): Target path
- `-t, --task` (required): Task name
- `-r, --root_path`: Project root path
- `-d, --debug`: Enable debug logging

## Usage Examples

### Basic Workflow

1. **Check variables:**
   ```bash
   fargate-td variables -p myapp/production
   ```

2. **Generate and review task definition:**
   ```bash
   fargate-td generate -p myapp/production -t web
   ```

3. **Deploy with specific version:**
   ```bash
   fargate-td deploy -p myapp/production -t web -v"Version=1.2.3"
   ```

4. **Monitor deployment:**
   ```bash
   fargate-td watch -p myapp/production -t web
   ```

### Variable Override Examples

```bash
# Single variable
fargate-td deploy -p myapp/dev -t api -v"Debug=true"

# Multiple variables
fargate-td deploy -p myapp/dev -t api -v"Version=1.0.0,Debug=true,Port=9000"
```

### Environment-Specific Deployments

```bash
# Development
fargate-td deploy -p myapp/development -t web -v"Version=dev-123"

# Staging  
fargate-td deploy -p myapp/staging -t web -v"Version=v1.0.0-rc1"

# Production
fargate-td deploy -p myapp/production -t web -v"Version=v1.0.0"
```

## Advanced Features

### Variable Precedence

Variables are merged with the following precedence (highest to lowest):

1. Command-line variables (`-v "key=value"`)
2. Task-container-specific variables
3. Container-specific variables
4. Task-level variables

### Template Processing

- Templates use Go's `text/template` syntax
- Files with `.tpl` extension are processed as templates
- Variables are available as `{{.VariableName}}`
- Missing variables cause template processing to fail

### Deployment Process

1. Generate task definition from overlays and templates
2. Register task definition with AWS ECS
3. Load deploy configuration to find services and cron jobs
4. Show color-coded diff of changes
5. Update ECS services (unless `--td-only`)
6. Update CloudWatch Events rules (unless `--td-only`)

### Monitoring

The watch command monitors deployment with:
- Default timeout: 10 minutes
- Check interval: 10 seconds
- Status reporting: Deployed, DeployFailed, Error, or Timeout

## Development

### Building

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Build
go build -o fargate-td ./cmd/fargate-td

# Clean
make clean
```

### Dependencies

```bash
# Download dependencies
go mod download

# Clean dependencies
go mod tidy
```

## License

This project is released under the MIT License.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request