# Command Reference

Detailed documentation for all forge commands.

**Quick Navigation:** [publish](#publish) | [meta](#meta) | [validate](#validate) | [empty](#empty) | [ping](#ping) | [list](#list) | [status](#status) | [output](#output) | [config](#config)

---

## Manage Project Metadata

### publish

Create a metadata upload job for your project.

**Usage:**
```bash
forge publish <github_personal_access_token> [--remote REMOTE_NAME]
```

**What it does:**

This is the main command you'll use. It dispatches a Sower job on the CALYPR platform that:
1. Validates your GitHub token
2. Clones your git repository
3. Generates FHIR metadata from your git-drs tracked files
4. Validates the metadata against FHIR schemas
5. Uploads the metadata to CALYPR

**Example:**
```bash
$ forge publish ghp_abc123def456

Using remote: production
Uid: job-xyz789-abc123   Name: fhir_import_export   Status: Pending
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**Common errors:**
- `Error: invalid token` - Your GitHub token is expired or missing the `repo` scope
- `Error: repository has no origin` - Push your repository to GitHub first
- `Error: could not locate remote` - Run this in a git-drs initialized repository

**When to use:** After pushing new or updated files via git-drs, run this to make them discoverable on CALYPR.

---

### meta

Generate FHIR metadata locally for debugging.

**Usage:**
```bash
forge meta [--remote REMOTE_NAME]
```

**What it does:**

Generates FHIR metadata files locally in the `META/` directory. This is useful for debugging what metadata will be created, but it's not required for normal workflows (metadata is generated automatically during the publish job).

The command:
1. Queries your CALYPR project for all DRS objects
2. Reads git-lfs tracked files in your repository
3. Matches them by SHA256 hash
4. Creates DocumentReference resources (one per file)
5. Creates Directory resources (one per folder)
6. Creates or updates the ResearchStudy resource (one per project)
7. Writes NDJSON files to `META/`

**Example:**
```bash
$ forge meta

Loaded existing ResearchStudy from ./META/ResearchStudy.ndjson with ID abc123...
Processed 15 records
Finished writing all DocumentReference records.
Finished writing all Directory records.
```

**Output structure:**
```
your-repo/
├── META/
│   ├── DocumentReference.ndjson
│   ├── Directory.ndjson
│   └── ResearchStudy.ndjson
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** When you want to inspect metadata before publishing, or debug why validation is failing.

---

### validate

Validate metadata and configuration files.

#### validate data

Validate FHIR NDJSON metadata files against schemas.

**Usage:**
```bash
forge validate data [PATH] [--remote REMOTE_NAME]
```

**What it does:**

Checks that NDJSON files in the `META/` directory conform to FHIR R5 schemas. It validates:
- Required fields are present
- Field types are correct
- Values follow FHIR constraints
- JSON structure is valid

**Example (success):**
```bash
$ forge validate data

Validating NDJSON files in META/...
✓ META/DocumentReference.ndjson (15 resources validated)
✓ META/Directory.ndjson (8 resources validated)
✓ META/ResearchStudy.ndjson (1 resource validated)

All files valid!
```

**Example (error):**
```bash
$ forge validate data

Error in META/DocumentReference.ndjson line 3:
  Missing required field: status
Validation failed.
```

**Arguments:**
- `PATH` - Path to NDJSON files directory (default: `./META`)

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** Before publishing, or when debugging failed publish jobs.

---

#### validate config

Validate explorer configuration files.

**Usage:**
```bash
forge validate config [PATH] [--remote REMOTE_NAME]
```

**What it does:**

Validates JSON configuration files used by the CALYPR explorer frontend. Checks that the structure matches the expected schema.

**Example:**
```bash
$ forge validate config ./CONFIG/my-project.json

✓ Configuration valid
```

**Arguments:**
- `PATH` - Path to config file (default: `./CONFIG`)

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** After generating or editing explorer config files.

---

#### validate edge

Check for orphaned edges in metadata graph.

**Usage:**
```bash
forge validate edge [PATH] [--remote REMOTE_NAME] [--export-vertices] [--export-edges]
```

**What it does:**

Validates that all references between FHIR resources are valid. For example, checks that DocumentReference resources reference valid Directory resources, and that all Directory references point to existing directories.

**Example:**
```bash
$ forge validate edge

Checking graph integrity...
✓ No orphaned edges found
All references valid.
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)
- `--export-vertices` - Export all vertices to a file for inspection
- `--export-edges` - Export all edges to a file for inspection

**When to use:** When debugging complex directory structures or reference issues.

---

### empty

Remove all metadata for your project from CALYPR.

**Usage:**
```bash
forge empty [--remote REMOTE_NAME]
```

**What it does:**

Dispatches a Sower job that deletes all FHIR metadata for your project from the CALYPR platform. Your data files remain in storage, but the metadata that makes them discoverable is removed.

**Example:**
```bash
$ forge empty

Using remote: production
Uid: job-delete-xyz789   Name: fhir_import_export   Status: Pending
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**Warning:** This operation cannot be undone. Use with caution.

**When to use:** When you need to clear out old metadata before re-publishing, or when decommissioning a project.

---

## Monitor Platform State

### ping

Check your connection and credentials.

**Usage:**
```bash
forge ping [--remote REMOTE_NAME]
```

**What it does:**

Verifies that:
- Your credentials are valid
- You can authenticate with the CALYPR platform
- Your project configuration is correct
- You have access to the configured storage buckets

**Example:**
```bash
$ forge ping
profile: production
username: researcher
endpoint: https://calypr-public.ohsu.edu
bucket_programs:
  bucket_name: program_name
your_access:
  /programs/my_program/projects/my_project: '*,create,delete,update,write-storage,file_upload'
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to ping (default: default_remote)

**When to use:** As a first step to verify your setup, or when debugging authentication issues.

---

### list

View all Sower jobs for your project.

**Usage:**
```bash
forge list [--remote REMOTE_NAME]
```

**What it does:**

Shows all processing jobs that have been dispatched for your project, including their status.

**Example:**
```bash
$ forge list

Using remote: production
Uid: job-abc123-456def   Name: fhir_import_export   Status: Completed
Uid: job-xyz789-012ghi   Name: fhir_import_export   Status: Running
Uid: job-old111-222jkl   Name: fhir_import_export   Status: Failed
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**Job statuses:**
- `Pending` - Job is queued, waiting to start
- `Running` - Job is currently executing
- `Succeeded` - Job completed successfully
- `Failed` - Job encountered an error

**When to use:** After running `forge publish` to check if your job has completed.

---

### status

Check the status of a specific job.

**Usage:**
```bash
forge status <UID> [--remote REMOTE_NAME]
```

**What it does:**

Shows the current status of a specific Sower job. Use the UID from the `forge list` or `forge publish` output.

**Example:**
```bash
$ forge status job-abc123-456def

Using remote: production
Uid: job-abc123-456def   Name: fhir_import_export   Status: Completed
```

**Arguments:**
- `UID` - The job UID (get this from `forge list` or `forge publish`)

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** To check on a specific job without listing all jobs.

---

### output

View logs from a specific job.

**Usage:**
```bash
forge output <UID> [--remote REMOTE_NAME]
```

**What it does:**

Retrieves and displays the output logs from a Sower job. This is essential for debugging failed jobs.

**Example (successful job):**
```bash
$ forge list

Using remote: production
Uid: job-abc123-456def   Name: fhir_import_export   Status: Completed
```

**Example (failed job):**
```bash
$ forge output job-xyz789-fail

Using remote: production
Logs:
Cloning repository...
Generating metadata...
Error: Validation failed for META/DocumentReference.ndjson line 5
Missing required field: status
Job failed.
```

**Arguments:**
- `UID` - The job UID (get this from `forge list` or `forge publish`)

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** When a job fails, or when you need detailed information about what happened during processing.

---

## Configure Portal Frontend

### config

Generate an explorer configuration template.

**Usage:**
```bash
forge config [--remote REMOTE_NAME]
```

**What it does:**

Creates a skeleton configuration file for the CALYPR explorer frontend. This file controls how your project appears in the CALYPR data portal.

The command generates a template at `CONFIG/[projectID].json` that you can customize.

**Example:**
```bash
$ forge config

Using remote: production
Created configuration template at CONFIG/my-project-123.json
```

**Output:**
```
your-repo/
├── CONFIG/
│   └── my_program-my_project.json
```

**Flags:**
- `--remote`, `-r` - Specify which CALYPR remote to use (default: default_remote)

**When to use:** When setting up a new project and you need to configure how it appears in the CALYPR portal.

---

## Global Flags

All commands support these flags:

- `--remote`, `-r` - Specify which git-drs remote to use (dev, staging, production, etc.)
- `--help`, `-h` - Show help for a command

**Examples:**
```bash
forge publish ghp_token --remote staging
forge ping --remote dev
forge list --remote production
```