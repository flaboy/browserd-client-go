package browserd

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateSnapshotResult checks the observation envelope, not page completeness.
func ValidateSnapshotResult(snapshot *SnapshotResult) error {
	if snapshot == nil || strings.TrimSpace(snapshot.SnapshotID) == "" {
		return fmt.Errorf("snapshotId is required")
	}
	raw, ok := snapshot.Page["url"].(string)
	if !ok {
		return fmt.Errorf("snapshot page URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("snapshot page URL must be HTTP(S)")
	}
	if groups, ok := snapshot.Page["groups"].(map[string]any); !ok || groups == nil {
		return fmt.Errorf("snapshot page groups must be an object")
	}
	return nil
}
