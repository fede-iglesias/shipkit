package frontmatter

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-yaml"
)

// EnsureType injects `type: <typ>` into yamlPart if the "type" key is absent.
// If "type" is already present with any value, it is preserved unchanged.
//
// The function parses the YAML into a map, checks for the "type" key, and if
// absent, prepends "type: <typ>\n" to the original bytes (preserving all other
// key order as-is). This avoids re-marshaling the full document and thus
// preserves comments and formatting as much as possible.
//
// Common use case: normalizing legacy frontmatter documents that were created
// before a "type" field was mandated in the schema.
//
// Returns the modified YAML bytes and any parse error.
// On success the returned bytes always end with a newline.
func EnsureType(yamlPart []byte, typ string) ([]byte, error) {
	// Empty YAML: just inject the type.
	if len(bytes.TrimSpace(yamlPart)) == 0 {
		return []byte(fmt.Sprintf("type: %s\n", typ)), nil
	}

	// Parse into a generic map to check for type key.
	var m map[string]any
	if err := yaml.Unmarshal(yamlPart, &m); err != nil {
		return nil, fmt.Errorf("frontmatter: EnsureType: parse YAML: %w", err)
	}

	if _, hasType := m["type"]; hasType {
		// Type already present: return unchanged.
		return yamlPart, nil
	}

	// Inject type at the start, preserving everything else.
	prefix := fmt.Sprintf("type: %s\n", typ)
	out := make([]byte, 0, len(prefix)+len(yamlPart))
	out = append(out, []byte(prefix)...)
	out = append(out, yamlPart...)
	return out, nil
}
