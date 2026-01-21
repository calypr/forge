# Getting Started with Forge

Forge manages FHIR metadata for datasets in the CALYPR Gen3 platform. It works alongside git-drs to make your data files discoverable and searchable through the CALYPR platform.

## Prerequisites

Before using forge, you'll need:

- Git DRS installed and configured (see the [git-drs documentation](https://github.com/calypr/git-drs/blob/main/README.md))
- A git-drs repository with data files already pushed to CALYPR
- CALYPR credentials set up through git-drs
- A GitHub Personal Access Token with `repo` scope ([create one here](https://github.com/settings/tokens))
- Go 1.21+ to build from source

## Installation

```bash
# Clone the repository and build
git clone https://github.com/calypr/forge.git
cd forge
go build -o forge

# Optionally, move to your PATH
sudo mv forge /usr/local/bin/

# Verify it works
forge --help
```

## Quick Start

Here's the basic flow to publish your dataset metadata:

```bash
# First, check your connection to CALYPR
forge ping

# Publish metadata (this kicks off a processing job)
forge publish ghp_your_github_token_here

# Monitor the job status
forge list  # provides you the job UID
forge status <job-uid>
forge output <job-uid>
```

Once the job succeeds, your dataset will be searchable onthe CALYPR platform!

## What Forge Does

Forge provides three main capabilities:

### 1. Manage Project Metadata
- **`forge publish <token>`** - Generate and upload FHIR metadata to CALYPR
- **`forge empty`** - Remove all metadata for your project from CALYPR
- **`forge meta`** - Preview what metadata will be generated (for debugging)
- **`forge validate data`** - Check that metadata is valid before publishing

### 2. Monitor Platform State
- **`forge ping`** - Verify your connection and credentials
- **`forge list`** - See all your processing jobs
- **`forge status <uid>`** - Check if a specific job succeeded or failed
- **`forge output <uid>`** - View detailed logs from a job

### 3. Configure Portal Frontend
- **`forge config`** - Generate a template configuration file for the CALYPR explorer page

## Common Workflows

### Publishing Your First Dataset

Make sure your files are already tracked and pushed through git-drs:

```bash
# Verify files are tracked
git lfs ls-files

# Publish the metadata
forge publish ghp_your_token_here

# Check the job status
forge list
# You'll see: Uid: job-abc123   Name: fhir_import_export   Status: Succeeded
```

### Adding More Files Later

When you add new data files, just push them with git-drs via git and re-publish:

```bash
# Add and push new files through git-drs
git add new-data/*.fastq.gz
git commit -m "Add more sequencing data"
git push origin main

# Update the metadata
forge publish ghp_token
```

### Debugging Metadata Before Publishing

If you want to see what metadata will be generated before publishing:
```bash
# Generate metadata locally
forge meta


# Look at what was created
ls -la META/
cat META/DocumentReference.ndjson | jq .
```

If you are supplying your own metadata and want to validate it before publishing:

```sh
# Validate it
forge validate data
```

### Working with Multiple Environments

If you need to push to a different environment:

```bash
# Specify which remote to use
forge ping --remote dev
forge publish ghp_token --remote staging
forge list --remote production
```

## How It Works

Here's what happens under the hood:

1. Your data files are tracked with git-drs and uploaded to CALYPR storage
2. You run `forge publish` which dispatches a job in the background
3. The job clones your repository, generates FHIR metadata, and uploads it to CALYPR
4. Once the job completes, your dataset appears in the CALYPR platform

**Important:** Metadata generation happens on the server during the publish job. The `forge meta` command is just for local debugging.

## What Gets Generated

Forge creates three types of FHIR R5 resources:

- **DocumentReference** - One for each file, with metadata like size, hash, URL, and creation date
- **Directory** - One for each folder, showing the directory structure
- **ResearchStudy** - One for the entire project, linking everything together

These resources are stored as NDJSON files in the `META/` directory.

## Troubleshooting

**"Error: could not locate remote"**

You need to run forge in a git-drs initialized repository:
```bash
git drs remote list
```

**"Error: no credentials found"**

Git-drs needs to be configured with your Gen3 credentials:
```bash
git drs remote add gen3 production \
    --cred ~/.gen3/credentials.json \
    --url https://calypr-public.ohsu.edu \
    --project my-project \
    --bucket my-bucket
```

**Job shows "Failed" status**

Check the logs to see what went wrong:
```bash
forge output <job-uid>

# Common issues:
# - Validation errors: try running `forge validate data` locally
# - Missing files: make sure `git push` completed successfully
# - Credential problems: verify with `forge ping`
```

## Next Steps

- [Commands Reference](commands.md) - Detailed documentation for all commands
- [Configuration Guide](configuration.md) - Understanding git-drs configuration
- [Metadata Structure](metadata.md) - Deep dive into FHIR resources
