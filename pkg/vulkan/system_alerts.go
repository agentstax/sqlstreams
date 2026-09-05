package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/alert"
)

// SystemAlertsHandle names the installation-wide alerts resource, holding no
// database row.
type SystemAlertsHandle struct {
	client *Client
}

// Alerts returns the installation-wide alerts handle. It performs no I/O.
func (s *SystemHandle) Alerts() *SystemAlertsHandle {
	return &SystemAlertsHandle{client: s.client}
}

// Definitions returns every Vulkan built-in alert definition ordered by VK
// code. It performs no I/O.
func (s *SystemAlertsHandle) Definitions() []AlertDefinition {
	return alert.Definitions()
}

// Latest returns the current alert per (name, owner) across the
// installation, active or resolved, ordered by message key.
func (s *SystemAlertsHandle) Latest(ctx context.Context) ([]*Alert, error) {
	stored, err := s.client.admin.ListAlerts(ctx)
	if err != nil {
		return nil, err
	}
	return unwrapMessages(stored), nil
}

// Alert names one alert owned by the system. It performs no I/O. No
// built-in is system-owned today, so this is the handle's only read.
func (s *SystemAlertsHandle) Alert(name string) *AlertHandle {
	return newAlertHandle(s.client, name, "", "")
}
