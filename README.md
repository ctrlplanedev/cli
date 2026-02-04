# CtrlC CLI

The official command-line interface for [Ctrlplane](https://ctrlplane.dev) - a modern platform for managing deployment environments and infrastructure resources.

## Overview

CtrlC (`ctrlc`) is a powerful CLI tool that enables you to:

- 🔄 **Sync infrastructure resources** from multiple cloud providers and platforms into CtrlPlane
- 📦 **Manage deployments** across environments with declarative configuration
- 🤖 **Run deployment agents** to execute jobs and workflows
- 🔍 **Query and manage resources** via intuitive API commands
- 📝 **Apply infrastructure as code** using YAML configurations

## Installation

### Using Go Install

```bash
go install github.com/ctrlplanedev/cli/cmd/ctrlc@latest
```

### Building from Source

```bash
git clone https://github.com/ctrlplanedev/cli.git
cd cli
make build
# Binary will be available at ./bin/ctrlc
```

### Using Make Install

```bash
make install
```

## Configuration

CtrlC can be configured using a config file, environment variables, or command-line flags.

### Config File

By default, CtrlC looks for a config file at `$HOME/.ctrlc.yaml`. You can specify a custom location:

```bash
ctrlc --config /path/to/config.yaml <command>
```

Example `~/.ctrlc.yaml`:

```yaml
url: https://app.ctrlplane.dev
api-key: your-api-key-here
workspace: your-workspace-id
```

### Environment Variables

```bash
export CTRLPLANE_URL="https://app.ctrlplane.dev"
export CTRLPLANE_API_KEY="your-api-key-here"
export CTRLPLANE_WORKSPACE="your-workspace-id"
export CTRLPLANE_CLUSTER_IDENTIFIER="my-cluster"
```

### Command-Line Flags

```bash
ctrlc --url https://app.ctrlplane.dev \
      --api-key your-api-key \
      --workspace your-workspace-id \
      <command>
```

### Setting Configuration Values

Use the `config set` command to persist configuration:

```bash
ctrlc config set url https://app.ctrlplane.dev
ctrlc config set api-key your-api-key
ctrlc config set workspace your-workspace-id
```

### Contexts (use-context)

Define named contexts in your config file and switch between them (similar to
`kubectl config use-context`):

```yaml
contexts:
  wandb:
    url: https://ctrlplane.wandb.io
    api-key: your-api-key
    workspace: wandb
  local:
    url: http://localhost:5173
    api-key: local-api-key
    workspace: test
current-context: wandb
```

Switch contexts:

```bash
ctrlc config use-context wandb
```

## Commands

### Agent

Run deployment agents to execute jobs and workflows:

```bash
# Start an agent
ctrlc agent run
```

### Sync

Synchronize infrastructure resources from various providers into CtrlPlane:

#### Cloud Providers

```bash
# AWS Resources
ctrlc sync aws ec2          # Sync EC2 instances
ctrlc sync aws eks          # Sync EKS clusters
ctrlc sync aws rds          # Sync RDS databases
ctrlc sync aws networks     # Sync VPCs and networks

# Google Cloud Resources
ctrlc sync google gke       # Sync GKE clusters
ctrlc sync google vms       # Sync Compute Engine VMs
ctrlc sync google cloudsql  # Sync Cloud SQL instances
ctrlc sync google cloudrun  # Sync Cloud Run services
ctrlc sync google buckets   # Sync Cloud Storage buckets
ctrlc sync google redis     # Sync Memorystore Redis
ctrlc sync google bigtable  # Sync Bigtable instances
ctrlc sync google secrets   # Sync Secret Manager secrets
ctrlc sync google projects  # Sync GCP projects
ctrlc sync google networks  # Sync VPCs and networks

# Azure Resources
ctrlc sync azure aks        # Sync AKS clusters
ctrlc sync azure networks   # Sync virtual networks
```

#### Kubernetes & Helm

```bash
# Kubernetes Resources
ctrlc sync kubernetes       # Sync Kubernetes resources
ctrlc sync vcluster        # Sync vcluster instances

# Helm Releases
ctrlc sync helm            # Sync Helm releases
```

#### Other Integrations

```bash
# Terraform
ctrlc sync terraform       # Sync Terraform workspaces

# GitHub
ctrlc sync github          # Sync GitHub resources

# Tailscale
ctrlc sync tailscale       # Sync Tailscale devices

# ClickHouse
ctrlc sync clickhouse      # Sync ClickHouse resources

# Salesforce
ctrlc sync salesforce      # Sync Salesforce data
```

#### Running Sync on an Interval

All sync commands support the `--interval` flag to run continuously:

```bash
# Sync every 5 minutes
ctrlc sync terraform --interval 5m

# Sync every hour
ctrlc sync kubernetes --interval 1h

# Sync every day
ctrlc sync aws eks --interval 1d
```

### Apply

Apply declarative YAML configurations to create and manage infrastructure:

```bash
ctrlc apply -f config.yaml
```

Example configuration file:

```yaml
systems:
  - name: my-app
    slug: my-app
    description: My application system

providers:
  - name: my-provider
    type: kubernetes

relationships:
  - type: depends-on
    source: resource-a
    target: resource-b

policies:
  - name: auto-deploy
    system: my-app
    rules:
      - condition: status == "ready"
        action: deploy
```

### API Commands

Direct API access for resource management:

#### Get Resources

```bash
# Get a specific resource
ctrlc api get resource <resource-id>

# List resources
ctrlc api get resources

# Get system information
ctrlc api get system <system-id>

# Get workspace information
ctrlc api get workspace
```

#### Create Resources

```bash
# Create a deployment version
ctrlc api create deploymentversion --deployment <id> --version <version>

# Create an environment
ctrlc api create environment --name <name> --system <system-id>

# Create a release
ctrlc api create release --deployment <id>

# Create a system
ctrlc api create system --name <name> --slug <slug>

# Create relationships
ctrlc api create relationship job-to-resource --job <job-id> --resource <resource-id>
ctrlc api create relationship resource-to-resource --source <id> --target <id>
```

#### Update Resources

```bash
# Update a deployment version
ctrlc api update deploymentversion <id> --status deployed

# Update a release
ctrlc api update release <id> --status completed

# Update a system
ctrlc api update system <id> --name <new-name>
```

#### Upsert Resources

```bash
# Upsert a resource (create or update)
ctrlc api upsert resource --identifier <id> --kind <kind>

# Upsert a deployment version
ctrlc api upsert deploymentversion --deployment <id> --version <version>

# Upsert a policy
ctrlc api upsert policy --name <name> --system <system-id>

# Upsert a release
ctrlc api upsert release --name <name>
```

#### Delete Resources

```bash
# Delete an environment
ctrlc api delete environment <environment-id>

# Delete a policy
ctrlc api delete policy <policy-id>

# Delete a resource
ctrlc api delete resource <resource-id>
```

### Run

Execute jobs and workflows:

```bash
# Execute a command
ctrlc run exec <command>

# List Kubernetes jobs
ctrlc run kubernetes jobs
```

### Version

Display version information:

```bash
ctrlc version
```

## Output Formats

Most commands support multiple output formats:

```bash
# JSON output (default)
ctrlc api get resource <id> --format json

# YAML output
ctrlc api get resource <id> --format yaml

# GitHub Actions output
ctrlc api get resource <id> --format github-action

# Custom Go template
ctrlc api get resource <id> --template '{{.name}}'
```

## Logging

Control logging verbosity with the `--log-level` flag:

```bash
ctrlc --log-level debug sync kubernetes
ctrlc --log-level warn api get resources
ctrlc --log-level error agent run
```

Available log levels: `debug`, `info`, `warn`, `error`

## Development

### Prerequisites

- Go 1.24.2 or later
- Make

### Building

```bash
# Build the binary
make build

# Install locally
make install

# Run tests
make test

# Run linter
make lint

# Format code
make format

# Clean build artifacts
make clean
```

### Project Structure

```
cli/
├── cmd/ctrlc/          # Main CLI entry point
│   └── root/           # Command implementations
│       ├── agent/      # Agent commands
│       ├── api/        # API commands
│       ├── apply/      # Apply command
│       ├── config/     # Configuration commands
│       ├── run/        # Run commands
│       ├── sync/       # Sync integrations
│       └── version/    # Version command
├── internal/           # Internal packages
│   ├── api/           # API client
│   ├── cliutil/       # CLI utilities
│   ├── kinds/         # Resource kinds
│   └── options/       # Common options
├── pkg/               # Public packages
│   ├── agent/         # Agent implementation
│   └── jobagent/      # Job agent
└── Makefile           # Build scripts
```

### Version Information

Version information is embedded at build time using ldflags:

```bash
make build
# or
go build -ldflags "-X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.Version=1.0.0" ./cmd/ctrlc
```

## GitHub Actions

The CLI includes GitHub Actions for CI/CD workflows. See the `actions/` directory for available actions.

### Get Resource Action

```yaml
- uses: ctrlplanedev/cli/actions/get-resource@main
  with:
    resource-id: ${{ env.RESOURCE_ID }}
    api-key: ${{ secrets.CTRLPLANE_API_KEY }}
```

## Docker

A Docker image is available for running CtrlC in containers:

```bash
docker build -f docker/Dockerfile -t ctrlc .
docker run ctrlc version
```

See `docker/README.md` for more details.

## Links

- **Website**: [https://ctrlplane.dev](https://ctrlplane.dev)
- **Documentation**: [https://docs.ctrlplane.dev](https://docs.ctrlplane.dev)
- **GitHub**: [https://github.com/ctrlplanedev/cli](https://github.com/ctrlplanedev/cli)

## License

See [LICENSE](LICENSE) file for details.

## Support

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/ctrlplanedev/cli).
