# Meta

Metadata handling for CALPYR data platform

## Workflow -- General Design Paramters

This repo is designed to produce git hook commands that take care of metadata additions / subtractions that are run before or after certain git commands like commit and push. Draft workflow currently:

## Example user workflow

```
git clone repo
forge init -- exactly same as git-drs init, just a wrapper around it
git add files
git commit -m "test" -- same as git-drs
git push origin main -- same as git-drs
forge publish [github personal access token]
```

To generate a personal access token for a github repo check these docs:
https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens

## Command descriptions

### ping

Same as ping in g3t

### meta

Generates metadata from non checked in .meta files. If .meta files are already checked in you can regen metadata with -r flag. This command is run as part of the pre-commit command

### validate

Validates metadata against the jsonschema in grip

### precommit

Runs meta init command then locates all .ndjson files in META directory and validates each file.

### publish

Validates that your Personal Access token exists and is valid
Packages together relevent information used to init the git repo in a remote job
Kicks off a sower job to process the metadata files that you have just pushed up

No git hook for publish, users are expected to run that themselves.
