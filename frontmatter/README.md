# frontmatter

YAML frontmatter parser and atomic writer for Markdown documents.

## When to use

Use this package when you have Markdown files with YAML metadata blocks
delimited by `---` lines, and you need to read, modify, or write that metadata
reliably. It preserves field order via `goccy/go-yaml` and writes files
atomically (write-temp-then-rename) so partial writes are never visible.

Common consumers: personal CLI tools that maintain a directory of typed
Markdown cards (people, tasks, ADRs, notes) with structured YAML frontmatter.

## Quick start

```go
import "github.com/fede-iglesias/shipkit/frontmatter"

// Read a document.
meta, body, err := frontmatter.ReadFile("person.md")
if err != nil {
    log.Fatal(err)
}

// Modify a field.
meta["status"] = "archived"

// Write it back atomically.
if err := frontmatter.WriteFile("person.md", meta, body); err != nil {
    log.Fatal(err)
}
```

## Common patterns

### Parse only (no file I/O)

```go
data := []byte("---\ntype: person\nslug: alice\n---\nBody.\n")

yamlPart, body, err := frontmatter.Split(data)
if err != nil {
    log.Fatal(err)
}

var meta struct {
    Type string `yaml:"type"`
    Slug string `yaml:"slug"`
}
if err := frontmatter.Unmarshal(yamlPart, &meta); err != nil {
    log.Fatal(err)
}
fmt.Println(meta.Slug) // alice
```

### Render with a typed struct

```go
type PersonMeta struct {
    Type   string   `yaml:"type"`
    Slug   string   `yaml:"slug"`
    Status string   `yaml:"status,omitempty"`
    Tags   []string `yaml:"tags,omitempty"`
}

meta := PersonMeta{Type: "person", Slug: "alice", Status: "active"}
body := []byte("Profile text.\n")

out, err := frontmatter.Marshal(meta, body)
if err != nil {
    log.Fatal(err)
}
// out: "---\ntype: person\nslug: alice\nstatus: active\n---\nProfile text.\n"
```

### Read directly into a struct

```go
type PersonMeta struct {
    Type string `yaml:"type"`
    Slug string `yaml:"slug"`
}

var meta PersonMeta
body, err := frontmatter.ReadFileInto("person.md", &meta)
if err != nil {
    log.Fatal(err)
}
fmt.Println(meta.Slug)
```

### Normalize: inject missing type field

```go
yaml := []byte("slug: alice\nstatus: active\n")
normalized, err := frontmatter.EnsureType(yaml, "person")
if err != nil {
    log.Fatal(err)
}
// normalized: "type: person\nslug: alice\nstatus: active\n"
```

### Inject and write atomically

```go
meta, body, err := frontmatter.ReadFile("legacy.md")
if err != nil {
    log.Fatal(err)
}
// Ensure type is present before writing.
yamlBytes, _ := yaml.Marshal(meta)
yamlBytes, err = frontmatter.EnsureType(yamlBytes, "note")
if err != nil {
    log.Fatal(err)
}
var normalized map[string]any
frontmatter.Unmarshal(yamlBytes, &normalized)
frontmatter.WriteFile("legacy.md", normalized, body)
```

## Gotchas

- **Field order depends on `goccy/go-yaml`, not stdlib `gopkg.in/yaml`.**
  `encoding/json`-style maps do not preserve insertion order. Use a typed
  struct with yaml tags if field order matters in the output.

- **WriteFile uses os.Rename for atomicity.** On Linux/macOS within the same
  filesystem this is atomic. Cross-device renames are not atomic; use
  `WriteFileWithRename` with a custom rename function if you need that.

- **ErrNoFrontmatter is a sentinel.** Use `errors.Is(err, frontmatter.ErrNoFrontmatter)`
  to distinguish missing frontmatter from other I/O errors.

- **Split normalizes line endings.** CRLF is converted to LF before parsing.
  The body is returned with LF line endings regardless of the input.

## Godoc

https://pkg.go.dev/github.com/fede-iglesias/shipkit/frontmatter
