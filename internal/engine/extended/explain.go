package extended

// PathsFor lists the variable paths a template references, deduplicated and in
// first-appearance order, keeping only the paths replaces selects. A nil
// predicate keeps every path. An operator's word is a template of its own, so
// ${A:-${B}} references both A and B; when the outer A is excluded the renderer
// leaves the whole token literal, so the word is not walked. The render layer
// uses it for --explain and --report=unused; it never resolves values, so it
// cannot change rendering.
func (e *Engine) PathsFor(src []byte, replaces func(string) bool) ([]string, error) {
	var paths []string
	seen := make(map[string]bool)
	err := walkReferences(src, func(ref reference) (bool, error) {
		if !ref.Bare && ref.Path != "" && replaces != nil && !replaces(ref.Path) {
			return false, nil
		}
		if ref.Path != "" && !seen[ref.Path] && (replaces == nil || replaces(ref.Path)) {
			seen[ref.Path] = true
			paths = append(paths, ref.Path)
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}
