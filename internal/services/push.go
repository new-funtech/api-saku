package services

import (
	"strings"

	"github.com/google/uuid"
)

type PushNotifier interface {
	SendToUserAsync(userID uuid.UUID, title, body string, data map[string]string)
}

// pushBody keeps every line of the body (sentence, detail, reason), one per row, so the expanded
// notification shows the whole thing and the collapsed one its first line.
func pushBody(body string) string {
	lines := make([]string, 0, 3)
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
func pushData(kind string, id uuid.UUID) map[string]string {
	return map[string]string{"type": kind, "id": id.String()}
}
