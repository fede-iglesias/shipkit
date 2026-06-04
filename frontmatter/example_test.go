package frontmatter_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fede-iglesias/shipkit/frontmatter"
)

// ExampleSplit demonstrates splitting a frontmatter document into YAML and body parts.
func ExampleSplit() {
	doc := []byte("---\ntype: person\n---\nBody text.\n")
	yamlPart, body, err := frontmatter.Split(doc)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("yaml: %s", yamlPart)
	fmt.Printf("body: %s", body)
	// Output:
	// yaml: type: person
	// body: Body text.
}

// ExampleMarshal demonstrates assembling a frontmatter document from metadata and body.
func ExampleMarshal() {
	type Meta struct {
		Type string `yaml:"type"`
		Slug string `yaml:"slug"`
	}
	meta := Meta{Type: "person", Slug: "alice"}
	body := []byte("Body text.\n")

	out, err := frontmatter.Marshal(meta, body)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(string(out))
	// Output:
	// ---
	// type: person
	// slug: alice
	// ---
	// Body text.
}

// ExampleWriteFile demonstrates atomic write of a frontmatter document.
func ExampleWriteFile() {
	dir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "person.md")
	type Meta struct {
		Type string `yaml:"type"`
		Slug string `yaml:"slug"`
	}
	meta := Meta{Type: "person", Slug: "alice"}
	body := []byte("Profile text.\n")

	if err := frontmatter.WriteFile(path, meta, body); err != nil {
		fmt.Println("error:", err)
		return
	}

	// Read it back.
	got, gotBody, err := frontmatter.ReadFile(path)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("type:", got["type"])
	fmt.Println("slug:", got["slug"])
	fmt.Printf("body: %s", gotBody)
	// Output:
	// type: person
	// slug: alice
	// body: Profile text.
}

// ExampleEnsureType demonstrates injecting a "type" field into YAML that lacks one.
func ExampleEnsureType() {
	yaml := []byte("slug: alice\n")
	out, err := frontmatter.EnsureType(yaml, "person")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(string(out))
	// Output:
	// type: person
	// slug: alice
}
