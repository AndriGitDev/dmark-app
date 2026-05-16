package entitlements

type Plan string

const (
	PlanFree     Plan = "free"
	PlanMonitor  Plan = "monitor"
	PlanReporter Plan = "reporter"
	PlanBusiness Plan = "business"
)

type Limits struct {
	SavedDomains        int
	ScanCadenceMinutes  int
	ReportRetentionDays int
	AlertChannels       int
	APIAccess           bool
	Reporter            bool
	Teams               bool
}

func ForPlan(plan Plan) Limits {
	switch plan {
	case PlanBusiness:
		return Limits{SavedDomains: 250, ScanCadenceMinutes: 60, ReportRetentionDays: 730, AlertChannels: 5, APIAccess: true, Reporter: true, Teams: true}
	case PlanReporter:
		return Limits{SavedDomains: 50, ScanCadenceMinutes: 240, ReportRetentionDays: 365, AlertChannels: 3, APIAccess: true, Reporter: true}
	case PlanMonitor:
		return Limits{SavedDomains: 10, ScanCadenceMinutes: 720, ReportRetentionDays: 30, AlertChannels: 1}
	default:
		return Limits{SavedDomains: 3, ScanCadenceMinutes: 1440, ReportRetentionDays: 0}
	}
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
	PermissionManageBilling Permission = "billing:manage"
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
		return []Permission{PermissionViewDomain, PermissionManageDomain, PermissionManageBilling, PermissionManageMembers, PermissionManageAPIKeys, PermissionViewAuditLog}
	case RoleAdmin:
		return []Permission{PermissionViewDomain, PermissionManageDomain, PermissionManageMembers, PermissionManageAPIKeys, PermissionViewAuditLog}
	case RoleMember:
		return []Permission{PermissionViewDomain, PermissionManageDomain}
	default:
		return []Permission{PermissionViewDomain}
	}
}
