package domain

type Role string

const (
	RoleEngineer Role = "engineer"
	RoleReviewer Role = "reviewer"
	RoleDeployer Role = "deployer"
)

func RequireRole(actual Role, allowed ...Role) error {
	for _, role := range allowed {
		if actual == role {
			return nil
		}
	}
	return ErrUnauthorized
}
