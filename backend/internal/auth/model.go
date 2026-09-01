package auth

import "strings"

type Role string

const (
	RoleLearner Role = "learner"
	RoleAdmin   Role = "admin"
)

type User struct {
	ID             string  `json:"id"`
	Email          string  `json:"email"`
	Role           Role    `json:"role"`
	DefaultLevelID *string `json:"defaultLevelId"`
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidEmail(email string) bool {
	at := strings.Index(email, "@")
	return at > 0 && at < len(email)-1 && !strings.Contains(email, " ")
}

func ValidRole(r string) bool { return r == string(RoleLearner) || r == string(RoleAdmin) }
