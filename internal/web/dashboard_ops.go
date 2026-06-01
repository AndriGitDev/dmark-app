package web

import (
	"net/http"
	"time"

	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/ops"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

func recordDashboardRecheckOperationalEvent(r *http.Request, store OperationalEventStore, organizationID string, domain domains.Domain, status int, kind string, severity ops.Severity, message string, extra map[string]any) {
	metadata := map[string]any{
		"organization_id": organizationID,
		"domain_id":       domain.ID,
		"domain":          domain.Domain,
	}
	for key, value := range extra {
		metadata[key] = value
	}
	recordDashboardOperationalEvent(r, store, organizationID, kind, severity, message, "domain:"+domain.ID, status, metadata)
}

func recordDashboardSPFRouteOperationalEvent(r *http.Request, store OperationalEventStore, organizationID string, domain domains.Domain, route spfroutes.Route, status int, kind string, severity ops.Severity, message string, extra map[string]any) {
	metadata := map[string]any{
		"organization_id": organizationID,
		"domain_id":       domain.ID,
		"domain":          domain.Domain,
		"route_id":        route.ID,
		"sources_count":   len(route.Sources),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	resource := "domain:" + domain.ID
	if route.ID != "" {
		resource = "spf_route:" + route.ID
	}
	recordDashboardOperationalEvent(r, store, organizationID, kind, severity, message, resource, status, metadata)
}

func recordDashboardOperationalEvent(r *http.Request, store OperationalEventStore, organizationID string, kind string, severity ops.Severity, message string, resource string, status int, metadata map[string]any) {
	if store == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["method"] = r.Method
	metadata["path"] = r.URL.Path
	metadata["http_status"] = status
	metadata["request_id"] = RequestID(r.Context())
	event, err := ops.NewEvent(kind, severity, message, time.Now().UTC())
	if err != nil {
		return
	}
	event.Actor = "organization:" + organizationID
	event.Resource = resource
	event.Metadata = ops.SanitizeMetadata(metadata)
	_, _ = store.CreateOperationalEvent(r.Context(), event)
}
