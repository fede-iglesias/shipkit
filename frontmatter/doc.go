// Package frontmatter provides YAML frontmatter parsing and atomic writing
// for Markdown documents. A frontmatter document looks like:
//
//	---
//	type: person
//	slug: alice
//	---
//	Body text goes here.
//
// The YAML block is delimited by lines containing only "---". The body is
// everything after the second delimiter.
//
// # Design
//
// The package is split into three concerns:
//
//   - Parsing (Split, Unmarshal, Marshal): pure byte-level operations that
//     separate YAML from body and round-trip field order via goccy/go-yaml.
//     Field order is preserved on struct types through struct tag ordering.
//
//   - File I/O (ReadFile, ReadFileInto, WriteFile, WriteFileWithRename):
//     file-backed helpers that combine parsing with atomic writes. WriteFile
//     uses a write-temp-then-rename strategy so partial writes are never
//     visible to readers.
//
//   - Normalization (EnsureType): mutation helpers that inject or preserve
//     metadata fields without re-marshaling the entire document.
//
// The package depends only on github.com/goccy/go-yaml, which preserves field
// order via an ordered-map representation. Unlike gopkg.in/yaml.v3, this
// library does not reorder map keys on round-trip.
//
// # Usage
//
// Parse a frontmatter document and re-render it with an updated field:
//
//	data, _ := os.ReadFile("person.md")
//
//	yamlPart, body, err := frontmatter.Split(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	var meta struct {
//	    Type string `yaml:"type"`
//	    Slug string `yaml:"slug"`
//	}
//	if err := frontmatter.Unmarshal(yamlPart, &meta); err != nil {
//	    log.Fatal(err)
//	}
//
//	meta.Slug = "alice-updated"
//	out, err := frontmatter.Marshal(meta, body)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(string(out))
//
// For a simpler read-modify-write cycle, use ReadFile + WriteFile directly:
//
//	meta, body, err := frontmatter.ReadFile("person.md")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	meta["status"] = "archived"
//	if err := frontmatter.WriteFile("person.md", meta, body); err != nil {
//	    log.Fatal(err)
//	}
//
// # See also
//
//   - [shipkit/lifecycle/migrations] for schema migration of frontmatter documents.
//   - [shipkit/store] for atomic file locking and path utilities.
package frontmatter
