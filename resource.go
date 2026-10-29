package nws

import "errors"

// validateResourceID rejects empty identifiers and dot segments, which PathEscape
// would leave unchanged and URL resolution would treat as navigation.
func validateResourceID(value string) error {
	if value == "" || value == "." || value == ".." {
		return errors.New("nws: non-empty resource identifier required (not . or ..)")
	}
	return nil
}
