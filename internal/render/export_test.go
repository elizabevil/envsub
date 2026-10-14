package render

import "io"

// ReadTemplateForTest exposes template reading to package tests.
func ReadTemplateForTest(paths []string, stdin io.Reader) ([]byte, error) {
	path := ""
	if len(paths) > 0 {
		path = paths[0]
	}
	return readTemplate(path, stdin)
}
