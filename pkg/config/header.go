package config

import (
	"bytes"

	"go.yaml.in/yaml/v3"
)

// varsKey is the key of the vars header.
const varsKey = "vars"

// header is the vars header of a file: a first YAML document that holds
// nothing but vars and ends with a "---" line, as in
//
//	vars:
//	  ARCH: x64
//	---
//	package:
//	  platform: {{ .ARCH }}
//
// A "---" line may also open the file. The header is plain YAML, so a
// template can read its variables before it is rendered.
type header struct {
	// root is the header's mapping, whose only key is vars.
	root *yaml.Node
	// text is the header up to and including its closing "---" line.
	text []byte
	// lines is the number of lines in text.
	lines int
}

// splitHeader returns the vars header of data, or nil when data has none.
// The body is data[len(h.text):].
func splitHeader(data []byte) *header {
	end := headerEnd(data)
	if end < 0 {
		return nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data[:end], &doc); err != nil {
		return nil
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode || len(root.Content) != 2 || root.Content[0].Value != varsKey {
		return nil
	}

	text := data[:end]

	return &header{root: root, text: text, lines: bytes.Count(text, []byte("\n"))}
}

// headerEnd returns the offset just past the "---" line that ends the
// first document of data, or -1 when the first document is not followed
// by one. A "---" line before any content opens the document instead.
func headerEnd(data []byte) int {
	content := false

	for pos := 0; pos < len(data); {
		next := bytes.IndexByte(data[pos:], '\n')
		if next < 0 {
			return -1
		}

		line := bytes.TrimRight(data[pos:pos+next], " \t\r")
		pos += next + 1

		switch {
		case bytes.Equal(line, []byte("---")) && content:
			return pos
		case bytes.Equal(line, []byte("---")):
			// The marker that opens the first document.
			content = true
		case len(bytes.TrimLeft(line, "\uFEFF \t")) > 0 && !bytes.HasPrefix(bytes.TrimLeft(line, "\uFEFF \t"), []byte("#")):
			content = true
		}
	}

	return -1
}
