# Forge

FHIR metadata management for CALYPR Gen3 data repositories.

Forge works alongside [git-drs](https://github.com/calypr/git-drs/blob/main/README.md) to generate and publish FHIR-compliant metadata, making your datasets discoverable on the CALYPR platform.

## Quick Start

```bash
# Verify your connection to CALYPR
forge ping

# Publish metadata to CALYPR
forge publish ghp_your_github_token

# Monitor the job
forge list
forge status <job-uid>
```

## What Forge Does

**1. Manage Project Metadata**
- `forge publish` - Generate and upload metadata to CALYPR
- `forge empty` - Remove project metadata
- `forge meta` - Preview metadata locally
- `forge validate` - Check metadata validity

**2. Monitor Platform State**
- `forge ping` - Check connection and credentials
- `forge list` - View all processing jobs
- `forge status` - Check specific job status
- `forge output` - View job logs

**3. Configure Portal Frontend**
- `forge config` - Generate a CALYPR explorer template

## Installation

```bash
git clone https://github.com/calypr/forge.git
cd forge
go build -o forge
sudo mv forge /usr/local/bin/
```

## Prerequisites

- Git DRS installed and configured
- Data files pushed to CALYPR via git-drs
- Gen3 credentials (configured through git-drs)
- GitHub Personal Access Token ([create token](https://github.com/settings/tokens))

## Documentation

- [Getting Started](docs/getting-started.md) - Setup and basic workflows
- [Command Reference](docs/commands.md) - Detailed command documentation
- [Configuration Guide](docs/configuration.md) - Git-drs configuration
- [Metadata Structure](docs/metadata.md) - Understanding FHIR resources

## Example Workflow

```bash
# Use git-drs to track and push files
git lfs track "*.fastq.gz"
git add data/sample.fastq.gz
git commit -m "Add sequencing data"
git push

# Publish metadata to CALYPR
forge publish ghp_abc123def456

# Monitor the job
forge list
# Uid: job-xyz789   Name: fhir_import_export   Status: Succeeded
```

## Support

Part of the CALYPR data commons ecosystem.

## Releasing

Forge uses [GoReleaser](https://goreleaser.com/) for automated builds and releases.

### Automated Releases
Every merge to the `main` branch automatically:
1. Creates a new patch version tag (e.g., `v0.1.5` -> `v0.1.6`).
2. Builds binaries for macOS and Linux.
3. Creates a GitHub Release with the compiled assets.

To trigger a different version bump via the GitHub Action, include one of the following in your merge commit message:
- `#major` for a major version bump.
- `#minor` for a minor version bump.
- `#patch` for a patch version bump.

### Manual Tagging
If you need to manually tag a version, use the provided helper script:
```bash
./bump-tag.sh --patch   # Increments v0.0.x
./bump-tag.sh --minor   # Increments v0.x.0
./bump-tag.sh --major   # Increments vx.0.0
```
Then push the tag to trigger the release workflow:
```bash
git push origin main --tags
```
