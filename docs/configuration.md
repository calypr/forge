# Configuration Guide

Forge uses Git DRS configuration for all its settings: no separate configuration needed.

## Configuration Files

### `.drs/config.yaml`

This is the main configuration file created by Git DRS. It contains:
- Remote server configurations (URLs, credentials, project IDs, buckets)
- Default remote settings
- Authentication profiles

Forge reads this file to determine which CALYPR instance to connect to and how to authenticate.

**Location:** `.drs/config.yaml` in your git repository root


### CALYPR Credentials

CALYPR credentials are provided by your platform administrator. You'll typically receive:
- A website URL
- A project ID
- Credentials (retrieved from /Profile page on the website)

These can be used in the git-drs repo to authenticate yourself

### Git Credentials

You can place those in a common place like a ~/.bash_profile or ~/.zshrc by adding the line

```sh
export GH_PAT=<paste-github-token-here>
```

you can then reference them when publishing projects

```sh
forge publish $GH_PAT
```

## Explorer Configuration

The `forge config` command generates a configuration template for the CALYPR explorer frontend. This is separate from the Git DRS configuration and controls how your project appears on the CALYPR platform.

```bash
forge config
```

This creates `CONFIG/[projectID].json` which you can customize to:
- Define visible fields
- Configure filters
- Set up shared filters across metadata
- Specify custom data visualizations

See the CALYPR documentation for details on explorer configuration options.

## See Also

- [Git DRS Documentation](https://github.com/calypr/git-drs/blob/main/README.md) - Complete Git DRS setup guide
- [Commands Reference](commands.md) - All forge commands
- [Getting Started](getting-started.md) - Quick start guide