package app

func commandAllowed(role, path string) bool {
	if res, ok := resources[path]; ok && res.Write != nil {
		return allowed(role, res.Write)
	}
	var roles []string
	switch path {
	case "book", "cancel-booking", "relationships", "subscriptions", "waitlist", "attendance", "branches", "rooms", "groups", "move-class", "cancel-class", "complete-class":
		roles = []string{"owner", "admin"}
	case "payment", "refund", "charges", "allocate":
		roles = []string{"owner", "cashier"}
	case "confirm-rights", "dispatch-artwork", "receive-artwork":
		roles = []string{"owner", "curator"}
	case "tariffs":
		roles = []string{"owner"}
	case "dispatch":
		roles = []string{"owner", "curator"}
	}
	return allowed(role, roles)
}
