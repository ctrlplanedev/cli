# CtrlC CLI

The official command-line interface for [Ctrlplane](https://ctrlplane.dev) - a modern platform for managing deployment environments and infrastructure resources.

## Overview

CtrlC (`ctrlc`) is a powerful CLI tool that enables you to:

- 🔄 **Sync infrastructure resources** from multiple cloud providers and platforms into CtrlPlane
- 📦 **Manage deployments** across environments with declarative configuration
- 🤖 **Run deployment agents** to execute jobs and workflows
- 🔍 **Query and manage resources** via intuitive API commands
- 📝 **Apply infrastructure as code** using YAML configurations

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
