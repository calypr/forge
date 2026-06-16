# FHIR Metadata Structure

Forge generates FHIR R5 (Fast Healthcare Interoperability Resources) metadata to describe your data files in a standardized format. This makes your datasets discoverable and searchable through the CALYPR portal.

## What is FHIR?

FHIR is a healthcare data standard that provides a common way to represent and exchange information. While it was designed for healthcare, its structured approach works well for any scientific data that needs rich metadata.

Forge uses FHIR because Gen3 can index and search FHIR resources, making your data discoverable through the portal's search interface.

## Generated Resources

Forge creates two types of FHIR resources:

### 1. DocumentReference (one per file)

Represents a single data file in your repository.

**What it contains:**
- Stable FHIR resource ID for the row itself
- Syfon object ID in `identifier[0]`
- File metadata: name, size, creation date
- Checksum values such as SHA256
- Storage URL from Syfon
- Reference to the parent `ResearchStudy`

**Example:**
```json
{
  "resourceType": "DocumentReference",
  "id": "abc123-def456-...",
  "identifier": [{
    "use": "official",
    "system": "https://calypr-public.ohsu.edu/BForePC",
    "value": "15d4dd21-618e-55ca-b325-860f58705d3a"
  }],
  "status": "current",
  "date": "2024-01-15T10:30:00Z",
  "content": [{
    "attachment": {
      "title": "sample_001.fastq.gz",
      "contentType": "application/gzip",
      "url": "s3://bforepc-prod/path/to/file.fastq.gz",
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
- `id` - Stable Forge-generated FHIR row ID
- `identifier[0]` - Official Syfon object ID used by downstream consumers
- `status` - Always "current" for active files
- `content.attachment` - File details (name, size, URL, type)
- `subject` - Links to the parent ResearchStudy

### 2. ResearchStudy (one per project)

Represents your entire project or dataset.

**What it contains:**
- Project-level identifier
- Gen3 project ID
- Project description
- Status
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
  "description": "Skeleton ResearchStudy for my-project-123"
}
```

**Key fields:**
- `id` - Generated from endpoint + project ID
- `identifier.value` - Your Gen3 project ID
- `status` - "active" for current projects
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
- `META/ResearchStudy.ndjson` - Project metadata

## How Files Are Mapped

When forge generates metadata, it follows this process:

### 1. Read existing metadata

If `META/DocumentReference.ndjson` already exists, Forge loads the current rows first. Those rows are treated as editable local metadata that may already contain category fields, subject references, and other annotations you want to preserve.

### 2. List Syfon project records

Forge lists the Syfon records for the configured `organization` and `project`. Syfon is the source of truth for object identity, object path, checksums, and access URL.

### 3. Join by SHA256

Forge joins existing local `DocumentReference` rows to Syfon records by SHA256. Forge does not push metadata back into Syfon. This is a read-Syfon plus update-local-NDJSON flow.

### 4. Rewrite the official Syfon identifier when needed

If a local row matches a Syfon object by SHA256 but `identifier[0]` is stale or missing, Forge rewrites the official identifier in memory before writing the file back out. This is intentional.

`identifier[0]` is the Syfon object ID boundary used by downstream services and the frontend download flow. Older metadata may still contain a path-style identifier or an outdated OID. Forge preserves the rest of the row, but it does not preserve a stale official identifier.

### 5. Generate missing `DocumentReference` rows

If a Syfon object exists for the project but no local `DocumentReference` row matches it by SHA256, Forge generates a new row.

## Duplicate SHA256 edge case

Some projects contain multiple distinct Syfon objects with the same SHA256. Common examples are small sidecar JSON files, offsets files, or copied artifacts that are byte-identical but live at different object paths.

This matters because a naive `map[sha] -> row` join is wrong for those projects. It causes one of these failures:

- only the first local row for that checksum gets updated
- later rows with the same checksum never receive their Syfon OID
- multiple Syfon objects with the same checksum collapse into one generated row

Forge handles this by:

- using SHA256 as the primary join key
- treating duplicate-checksum rows as a set, not a single record
- using the stored file path only to disambiguate which local row belongs to which Syfon object when the checksum is not unique

The important boundary is still Syfon object identity. The path is only a tie-breaker for duplicate hashes.

## What Forge Owns

In the current Syfon-native flow, Forge owns only:

- `META/DocumentReference.ndjson`
- `META/ResearchStudy.ndjson`

Forge no longer emits custom `Directory` resources and no longer adds a custom `rootDir` field to `ResearchStudy`.

## ID Generation

All resource IDs are deterministic, meaning the same input always produces the same ID. This ensures consistency across metadata updates.

**ID generation algorithm:**
```
ID = SHA1(SHA1(endpoint) + resource_path)
```

**Examples:**
- DocumentReference: `SHA1(SHA1(endpoint) + file_path)`
- ResearchStudy: `SHA1(SHA1(endpoint) + "ResearchStudy" + project_id)`

This approach ensures:
- IDs are globally unique
- The same file always gets the same ID
- No collisions between different resources

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

When you add or modify files and run `forge publish` again, Forge either uses the metadata provided or regenerates all metadata:

1. Existing `DocumentReference` rows are preserved and refreshed against current Syfon object identity
2. New files get new DocumentReference resources
3. Deleted files have their DocumentReferences removed
4. The `ResearchStudy` file is preserved and refreshed without custom directory fields

Because IDs are deterministic, the same files keep the same IDs across updates.


## See Also

- [Commands Reference](commands.md) - Using `forge meta` and `forge validate`
- [Getting Started](getting-started.md) - Basic workflow
- [FHIR R5 Specification](https://hl7.org/fhir/R5/) - Official FHIR documentation
- [Gen3 Documentation](https://gen3.org) - Gen3 platform details
