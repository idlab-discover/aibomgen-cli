package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func parseNonEmptyString(value string, field string) (string, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return "", fmt.Errorf("%s value is empty", field)
	}
	return s, nil
}

func parseOptionalString(value string) (string, error) {
	return strings.TrimSpace(value), nil
}

func parseTagsPreserveEmpty(value string, field string) ([]string, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("%s value is empty", field)
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}

func parseCommaList(value string, field string) ([]string, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("%s value is empty", field)
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid %s found", field)
	}
	return out, nil
}

func parseDatasetRefs(value string) ([]cdx.MLDatasetChoice, error) {
	refs, err := parseCommaList(value, "datasets")
	if err != nil {
		return nil, err
	}
	choices := make([]cdx.MLDatasetChoice, 0, len(refs))
	for _, ref := range refs {
		choices = append(choices, cdx.MLDatasetChoice{Ref: ref})
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("no valid dataset references found")
	}
	return choices, nil
}

func parseEthicalConsiderations(value string) ([]cdx.MLModelCardEthicalConsideration, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("ethicalConsiderations value is empty")
	}
	items := strings.Split(s, ",")
	ethics := []cdx.MLModelCardEthicalConsideration{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, mitigation, _ := strings.Cut(item, ":")
		name = strings.TrimSpace(name)
		if name != "" {
			ethics = append(ethics, cdx.MLModelCardEthicalConsideration{
				Name:               name,
				MitigationStrategy: strings.TrimSpace(mitigation),
			})
		}
	}
	if len(ethics) == 0 {
		return nil, fmt.Errorf("no valid ethical considerations found")
	}
	return ethics, nil
}

func parsePerformanceMetrics(value string) ([]cdx.MLPerformanceMetric, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("performanceMetrics value is empty")
	}
	metrics := []cdx.MLPerformanceMetric{}
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		mt, mv, _ := strings.Cut(pair, ":")
		mt = strings.TrimSpace(mt)
		if mt != "" {
			metrics = append(metrics, cdx.MLPerformanceMetric{Type: mt, Value: strings.TrimSpace(mv)})
		}
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no valid performance metrics found")
	}
	return metrics, nil
}

func parseProperties(value string) ([]cdx.Property, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("environmentalConsiderations value is empty")
	}
	props := []cdx.Property{}
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		name, val, ok := strings.Cut(pair, ":")
		name, val = strings.TrimSpace(name), strings.TrimSpace(val)
		if ok && name != "" && val != "" {
			props = append(props, cdx.Property{Name: name, Value: val})
		}
	}
	if len(props) == 0 {
		return nil, fmt.Errorf("no valid key:value pairs found in environmentalConsiderations")
	}
	return props, nil
}

func parseDataGovernance(value string) (*cdx.DataGovernance, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("governance value is empty")
	}

	governance := &cdx.DataGovernance{}
	hasGovernance := false

	// Parse format: "custodian:OrgName,steward:OrgName,owner:OrgName".
	// Or simpler: single value assumes custodian.
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		role, orgName, ok := strings.Cut(pair, ":")
		if ok {
			role = strings.ToLower(strings.TrimSpace(role))
			orgName = strings.TrimSpace(orgName)
		} else {
			// No role specified, default to custodian.
			role, orgName = "custodian", pair
		}

		if orgName == "" {
			continue
		}

		party := &[]cdx.ComponentDataGovernanceResponsibleParty{{
			Organization: &cdx.OrganizationalEntity{Name: orgName},
		}}
		switch role {
		case "custodian", "custodians":
			governance.Custodians = party
		case "steward", "stewards", "curated", "curatedby":
			governance.Stewards = party
		case "owner", "owners", "funded", "fundedby":
			governance.Owners = party
		default:
			continue
		}
		hasGovernance = true
	}

	if !hasGovernance {
		return nil, fmt.Errorf("no valid governance roles found")
	}

	return governance, nil
}
