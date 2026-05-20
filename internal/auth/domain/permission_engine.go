package domain

func Can(userRoles []string, action string, projectID string) bool {
	for _, role := range userRoles {
		if role == "admin" {
			return true
		}
		switch action {
		case "read":
			return true
		case "execute":
			return role == "pm" || role == "dev_lead" || role == "dev"
		case "approve", "review":
			return role == "pm" || role == "dev_lead"
		case "admin":
			return false
		}
	}
	return false
}
