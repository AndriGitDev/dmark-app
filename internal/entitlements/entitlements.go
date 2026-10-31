package entitlements

type Limits struct {
	SavedDomains        int
	ScanCadenceMinutes  int
	ReportRetentionDays int
	AlertChannels       int
	APIAccess           bool
	Reporter            bool
	Teams               bool
}

// DefaultLimits applies the same resource limits and capabilities to every organization.
func DefaultLimits() Limits {
	return Limits{SavedDomains: 250, ScanCadenceMinutes: 60, ReportRetentionDays: 730, AlertChannels: 5, APIAccess: true, Reporter: true, Teams: true}
}

func (limits Limits) AllowsSavedDomain(currentCount int) bool {
	return limits.SavedDomains <= 0 || currentCount < limits.SavedDomains
}

func (limits Limits) AllowsScanCadence(minutes int) bool {
	if limits.ScanCadenceMinutes <= 0 {
		return true
	}
	return minutes >= limits.ScanCadenceMinutes
}

func (limits Limits) AllowsReportRetention(days int) bool {
	if days <= 0 {
		return true
	}
	return limits.ReportRetentionDays > 0 && days <= limits.ReportRetentionDays
}

func (limits Limits) AllowsAlertChannels(currentCount int) bool {
	return limits.AlertChannels <= 0 || currentCount < limits.AlertChannels
}

func (limits Limits) AllowsReporterAccess() bool {
	return limits.Reporter
}

func (limits Limits) AllowsAPIAccess() bool {
	return limits.APIAccess
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type Permission string

const (
	PermissionViewDomain    Permission = "domain:view"
	PermissionManageDomain  Permission = "domain:manage"
	PermissionManageMembers Permission = "members:manage"
	PermissionManageAPIKeys Permission = "api_keys:manage"
	PermissionViewAuditLog  Permission = "audit_log:view"
)

func HasPermission(role Role, permission Permission) bool {
	permissions := permissionsForRole(role)
	for _, allowed := range permissions {
		if allowed == permission {
			return true
		}
	}
	return false
}

func permissionsForRole(role Role) []Permission {
	switch role {
	case RoleOwner:
		return []Permission{PermissionViewDomain, PermissionManageDomain, PermissionManageMembers, PermissionManageAPIKeys, PermissionViewAuditLog}
	case RoleAdmin:
		return []Permission{PermissionViewDomain, PermissionManageDomain, PermissionManageMembers, PermissionManageAPIKeys, PermissionViewAuditLog}
	case RoleMember:
		return []Permission{PermissionViewDomain, PermissionManageDomain}
	default:
		return []Permission{PermissionViewDomain}
	}
}
