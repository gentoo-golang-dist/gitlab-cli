package mrutils

// TruncateDiscussionID truncates a discussion ID to 8 characters with an ellipsis.
// If the ID is 8 characters or shorter, it is returned unchanged.
func TruncateDiscussionID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}
