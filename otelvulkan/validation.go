package otelvulkan

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/prometheus/otlptranslator"
)

// Rejection is family-wide: observations with the same original name export together.
func rejectedFamilies(families map[string][]*common.StoredMessage[metrics.Measurement]) map[string]string {
	rejected := make(map[string]string)
	familiesByExportName := make(map[string][]string)

	// Only families that can export participate in output-name collisions.
	for name, family := range families {
		if err := validateMetricFamily(family); err != nil {
			rejected[name] = err.Error()
			continue
		}

		measurement := family[0].Message
		exportName, err := translateExportName(name, measurement.Kind, measurement.Unit)
		if err != nil {
			rejected[name] = err.Error()
			continue
		}
		familiesByExportName[exportName] = append(familiesByExportName[exportName], name)
	}

	// No family wins a shared output name: reject every original name in the collision.
	for exportName, names := range familiesByExportName {
		if err := validateUniqueExportName(exportName, names); err != nil {
			for _, name := range names {
				rejected[name] = err.Error()
			}
		}
	}
	return rejected
}

func validateMetricFamily(family []*common.StoredMessage[metrics.Measurement]) error {
	first := family[0].Message
	for _, row := range family {
		measurement := row.Message
		if measurement.Kind != first.Kind || measurement.Unit != first.Unit {
			return errors.New("metric family has conflicting kind or unit")
		}
	}

	// Attribute keys must keep the same meaning across all observations in the family.
	labelNamer := otlptranslator.LabelNamer{UTF8Allowed: false}
	attributeNames := make(map[string]string)
	for _, row := range family {
		for _, key := range slices.Sorted(maps.Keys(row.Message.Attributes)) {
			translated, err := labelNamer.Build(key)
			if err != nil {
				return err
			}
			if strings.HasPrefix(translated, "otel_scope_") || strings.HasPrefix(translated, "__") {
				return errors.New("translated attribute name is reserved for exporter metadata")
			}
			if previous, found := attributeNames[translated]; found && previous != key {
				return errors.New("metric family has ambiguous translated attribute names")
			}
			attributeNames[translated] = key
		}
	}
	return nil
}

// translateExportName returns the translated name unless it is reserved for Vulkan or exporter metadata.
func translateExportName(name string, kind metrics.MetricKind, unit metrics.MetricUnit) (string, error) {
	metricNamer := otlptranslator.NewMetricNamer("", otlptranslator.UnderscoreEscapingWithSuffixes)
	metricType := otlptranslator.MetricType(otlptranslator.MetricTypeGauge)
	if kind == metrics.MetricKindCounter {
		metricType = otlptranslator.MetricTypeMonotonicCounter
	}
	translated, err := metricNamer.Build(otlptranslator.Metric{Name: name, Unit: string(unit), Type: metricType})
	if err != nil {
		return "", err
	}

	_, builtIn := diagnostic.GetMetric(name)
	switch {
	case name == metrics.MetricOTelSourceReadSuccess.Name || name == metrics.MetricOTelMeasurementsRejected.Name:
		return "", errors.New("exporter health cannot be supplied by retained measurements")
	case translated == "vulkan_otel_source_read_success" || translated == "vulkan_otel_measurements_rejected":
		return "", errors.New("translated metric name is reserved for exporter health")
	case translated == otlptranslator.TargetInfoMetricName || strings.HasPrefix(translated, "otel_scope_"):
		return "", errors.New("translated metric name is reserved for exporter metadata")
	case !builtIn && strings.HasPrefix(translated, "vulkan_"):
		return "", errors.New("custom metric name enters the Vulkan namespace")
	default:
		return translated, nil
	}
}

func validateUniqueExportName(exportName string, names []string) error {
	if len(names) < 2 {
		return nil
	}
	sortedNames := slices.Sorted(slices.Values(names))
	return fmt.Errorf("metric names %q translate to %q", sortedNames, exportName)
}
