package domain

import "strings"

func AccountClassification(planType string) string {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "":
		return "Unknown"
	case "team":
		return "Team"
	case "business", "enterprise", "education", "edu", "k12", "quorum", "free_workspace":
		return "Business"
	default:
		return "Personal"
	}
}
