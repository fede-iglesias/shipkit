package frontmatter

import (
	"fmt"
	"os"
)

// ReadFile reads a frontmatter document from the given path.
// It splits the document into YAML and body, then unmarshals the YAML into a
// generic map[string]any. This is convenient when the caller does not have a
// typed struct for the metadata.
//
// Returns:
//   - meta: the parsed YAML fields as a map. Never nil on success; an empty
//     frontmatter block returns an empty (non-nil) map.
//   - body: the raw body bytes after the closing "---" delimiter.
//   - err: ErrNoFrontmatter if the file has no delimiters, or a wrapped OS/YAML error.
func ReadFile(path string) (meta map[string]any, body []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("frontmatter: read file %q: %w", path, err)
	}

	yamlPart, body, err := Split(data)
	if err != nil {
		return nil, nil, fmt.Errorf("frontmatter: split %q: %w", path, err)
	}

	var m map[string]any
	if err := Unmarshal(yamlPart, &m); err != nil {
		return nil, nil, fmt.Errorf("frontmatter: unmarshal %q: %w", path, err)
	}
	if m == nil {
		m = make(map[string]any)
	}

	return m, body, nil
}

// ReadFileInto reads a frontmatter document from path and unmarshals the YAML
// into v (must be a pointer to a struct or compatible type).
//
// This is the typed variant of ReadFile: use it when you have a concrete struct
// that matches the document schema. Field matching follows yaml struct tags.
//
// Returns:
//   - body: the raw body bytes after the closing "---" delimiter.
//   - err: ErrNoFrontmatter if the file has no delimiters, or a wrapped OS/YAML error.
func ReadFileInto(path string, v any) (body []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: read file %q: %w", path, err)
	}

	yamlPart, body, err := Split(data)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: split %q: %w", path, err)
	}

	if err := Unmarshal(yamlPart, v); err != nil {
		return nil, fmt.Errorf("frontmatter: unmarshal into struct %q: %w", path, err)
	}

	return body, nil
}
