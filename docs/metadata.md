# FHIR Metadata Structure

Forge generates FHIR R5 (Fast Healthcare Interoperability Resources) metadata to describe your data files in a standardized format. This makes your datasets discoverable and searchable through the Gen3 portal.

## What is FHIR?

FHIR is a healthcare data standard that provides a common way to represent and exchange information. While it was designed for healthcare, its structured approach works well for any scientific data that needs rich metadata.

Forge uses FHIR because Gen3 can index and search FHIR resources, making your data discoverable through the portal's search interface.

## Generated Resources

Forge creates three types of FHIR resources:

### 1. DocumentReference (one per file)

Represents a single data file in your repository.

**What it contains:**
- Unique identifier (deterministic UUID based on file hash)
- DRS object ID for retrieving the file
- File metadata: name, size, MIME type, creation date
- Hash values: MD5, SHA256, SHA512
- Storage URL (DRS endpoint)
- Reference to the parent ResearchStudy

**Example:**
```json
{
  "resourceType": "DocumentReference",
  "id": "abc123-def456-...",
  "identifier": [{
    "system": "https://calypr-public.ohsu.edu/drs",
    "value": "drs://dg.4503/abc123..."
  }],
  "status": "current",
  "date": "2024-01-15T10:30:00Z",
  "content": [{
    "attachment": {
      "title": "sample_001.fastq.gz",
      "contentType": "application/gzip",
      "url": "drs://calypr-public.ohsu.edu/abc123...",
      "size": 1073741824,
      "creation": "2024-01-15T10:30:00Z"
    }
  }],
  "subject": {
    "reference": "ResearchStudy/project-xyz789"
  }
}
```

**Key fields:**
- `id` - Generated from SHA1 hash of endpoint + filename
- `identifier` - DRS object ID for file retrieval
- `status` - Always "current" for active files
- `content.attachment` - File details (name, size, URL, type)
- `subject` - Links to the parent ResearchStudy

### 2. Directory (one per folder)

Represents a folder in your project's directory structure.

**What it contains:**
- Unique identifier for the directory
- Directory name
- References to child directories and files
- Position in the hierarchy

**Example:**
```json
{
  "resourceType": "Directory",
  "id": "dir-abc123-...",
  "name": "sequencing-data",
  "child": [
    {"reference": "DocumentReference/abc123..."},
    {"reference": "DocumentReference/def456..."},
    {"reference": "Directory/subdir-xyz789..."}
  ]
}
```

**Key fields:**
- `id` - Generated from SHA1 hash of endpoint + directory path
- `name` - Folder name
- `child` - Array of references to files and subdirectories

**Note:** Directory is not a standard FHIR R5 resource - it's a custom extension used by Gen3 to represent file system structure.

### 3. ResearchStudy (one per project)

Represents your entire project or dataset.

**What it contains:**
- Project-level identifier
- Gen3 project ID
- Project description
- Status
- Reference to the root directory

**Example:**
```json
{
  "resourceType": "ResearchStudy",
  "id": "project-abc123",
  "identifier": [{
    "use": "official",
    "system": "https://calypr-public.ohsu.edu/my-project-123",
    "value": "my-project-123"
  }],
  "status": "active",
  "description": "Skeleton ResearchStudy for my-project-123",
  "rootDir": {
    "reference": "Directory/root-dir-id"
  }
}
```

**Key fields:**
- `id` - Generated from endpoint + project ID
- `identifier.value` - Your Gen3 project ID
- `status` - "active" for current projects
- `rootDir` - Custom extension linking to root Directory

**Note:** The `rootDir` field is a custom extension, not part of standard FHIR.

## File Format: NDJSON

Metadata is stored as NDJSON (Newline Delimited JSON) files:
- One JSON object per line
- Each line is a complete FHIR resource
- No commas between lines
- Files are stored in the `META/` directory

**Example NDJSON file:**
```
{"resourceType":"DocumentReference","id":"abc123","status":"current",...}
{"resourceType":"DocumentReference","id":"def456","status":"current",...}
{"resourceType":"DocumentReference","id":"ghi789","status":"current",...}
```

**Generated files:**
- `META/DocumentReference.ndjson` - All file metadata
- `META/Directory.ndjson` - All directory metadata
- `META/ResearchStudy.ndjson` - Project metadata

## How Files Are Mapped

When forge generates metadata, it follows this process:

### 1. Discover Files

Queries Gen3 IndexD to find all DRS objects in your project, then reads git-lfs tracked files in your local repository.

### 2. Match by Hash

Matches DRS objects to local files using SHA256 hashes. This ensures each file is correctly identified even if filenames change.

### 3. Generate DocumentReference

For each matched file, creates a DocumentReference resource with:
- File path and name from git-lfs
- Size and hashes from DRS object
- Creation date from git-lfs metadata
- DRS URL for retrieval

### 4. Build Directory Tree

Parses file paths to construct the directory hierarchy. Creates a Directory resource for each unique folder path.

### 5. Link Everything

Connects DocumentReferences to their parent Directories, Directories to parent Directories, and all top-level Directories to the ResearchStudy via the rootDir field.

## ID Generation

All resource IDs are deterministic, meaning the same input always produces the same ID. This ensures consistency across metadata updates.

**ID generation algorithm:**
```
ID = SHA1(SHA1(endpoint) + resource_path)
```

**Examples:**
- DocumentReference: `SHA1(SHA1(endpoint) + file_path)`
- Directory: `SHA1(SHA1(endpoint) + directory_path)`
- ResearchStudy: `SHA1(SHA1(endpoint) + "ResearchStudy" + project_id)`

This approach ensures:
- IDs are globally unique
- The same file always gets the same ID
- No collisions between different resources

## Custom Extensions

Forge adds some non-standard FHIR fields for Gen3 integration:

### rootDir (in ResearchStudy)

Links the ResearchStudy to the root Directory resource.

```json
{
  "resourceType": "ResearchStudy",
  "rootDir": {
    "reference": "Directory/root-id"
  }
}
```

This allows Gen3 to navigate the entire directory tree starting from the project level.

### Directory Resource

The entire Directory resource type is a custom extension. It's not part of FHIR R5, but follows FHIR conventions for structure and references.

## Validation

Forge validates metadata against FHIR R5 schemas to ensure:
- Required fields are present
- Field types are correct
- Values follow FHIR constraints
- References point to valid resources

**Run validation:**
```bash
forge validate data
```

**Common validation errors:**
- Missing required fields (status, id, resourceType)
- Invalid field types (string vs number)
- Invalid references (pointing to non-existent resources)
- Malformed dates or timestamps

## Updating Metadata

When you add or modify files and run `forge publish` again, Forge either uses the metadata provided or  regenerates all metadata:

1. Existing DocumentReferences are updated with new information
2. New files get new DocumentReference resources
3. Deleted files have their DocumentReferences removed
4. Directory structure is rebuilt to reflect current state

Because IDs are deterministic, the same files keep the same IDs across updates.


## See Also

- [Commands Reference](commands.md) - Using `forge meta` and `forge validate`
- [Getting Started](getting-started.md) - Basic workflow
- [FHIR R5 Specification](https://hl7.org/fhir/R5/) - Official FHIR documentation
- [Gen3 Documentation](https://gen3.org) - Gen3 platform details
