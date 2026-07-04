package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

// LineSummary returns a human-readable string for the estimate item, used by
// CSV/PDF exporters. Curated lines get service-specific formatting; legacy
// falls back to attribute list.
func LineSummary(it estimates.Item) string {
	switch it.LineType {
	case estimates.LineTypeCurated:
		if s := curatedSummary(it); s != "" {
			return s
		}
	}
	return legacySummary(it)
}

func curatedSummary(it estimates.Item) string {
	m, ok := it.UserInputSnapshot.(map[string]any)
	if !ok {
		return ""
	}
	switch it.ServiceKey {
	case "ec2":
		return fmt.Sprintf("EC2 · %v × %v in %s · %v h/mo · %v",
			m["count"], m["instanceType"], it.RegionCode, m["hoursPerMonth"], m["operatingSystem"])
	}
	// Generic curated fallback: sorted key=value list.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return fmt.Sprintf("%s · %s · %s", it.ServiceKey, it.RegionCode, strings.Join(pairs, ", "))
}

func legacySummary(it estimates.Item) string {
	keys := make([]string, 0, len(it.Attributes))
	for k := range it.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, it.Attributes[k]))
	}
	return fmt.Sprintf("%s · %s · %s", it.ServiceCode, it.RegionCode, strings.Join(pairs, ", "))
}
