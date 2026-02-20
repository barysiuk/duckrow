package cmd

import "strings"

// joinStrings concatenates string slices with ", " separator.
func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

// truncateSource returns the host/owner/repo portion of a canonical source.
func truncateSource(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) > 3 {
		return parts[0] + "/" + parts[1] + "/" + parts[2]
	}
	return source
}
