// Package endpoint provides shared resource-identifier validation.
package endpoint

import "errors"

// ValidateSegment rejects empty identifiers and dot segments, which PathEscape
// would leave unchanged and URL resolution would treat as navigation.
func ValidateSegment(value string) error {
	if value == "" || value == "." || value == ".." {
		return errors.New("nws: non-empty resource identifier required (not . or ..)")
	}
	return nil
}
