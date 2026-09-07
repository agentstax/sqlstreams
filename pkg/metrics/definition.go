package metrics

import (
	"slices"

	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// MetricDefinition is one Vulkan built-in metric's identity and metadata.
// It exists before any measurement is collected.
type MetricDefinition struct {
	Code          string                 `json:"code"` // the VK code its docs page lives under
	Name          string                 `json:"name"` // the wire name every measurement carries
	Kind          MetricKind             `json:"kind"`
	Unit          MetricUnit             `json:"unit"`
	Description   string                 `json:"description"`
	Scope         diagnostic.MetricScope `json:"scope"`          // which resource or collection a series describes
	AttributeKeys []string               `json:"attribute_keys"` // the attribute names every measurement of it carries
}

// Definitions returns Vulkan's built-in metric definitions ordered by VK code.
// With no scopes it returns the whole catalog; otherwise it returns definitions
// belonging to any requested scope.
func Definitions(scopes ...diagnostic.MetricScope) []MetricDefinition {
	requestedScopes := make(map[diagnostic.MetricScope]bool, len(scopes))
	for _, scope := range scopes {
		requestedScopes[scope] = true
	}

	registered := diagnostic.Metrics()
	definitions := make([]MetricDefinition, 0, len(registered))
	for _, metric := range registered {
		if len(requestedScopes) > 0 && !requestedScopes[metric.Scope] {
			continue
		}
		definitions = append(definitions, MetricDefinition{
			Code:          metric.Code,
			Name:          metric.Name,
			Kind:          MetricKind(metric.Kind),
			Unit:          MetricUnit(metric.Unit),
			Description:   metric.Description,
			Scope:         metric.Scope,
			AttributeKeys: slices.Clone(metric.AttributeKeys),
		})
	}
	return definitions
}
