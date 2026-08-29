package eviction

// IsNoEviction returns true if policy is noeviction or unknown.
func IsNoEviction(policy string) bool {
	return policy == "noeviction" || policy == ""
}
