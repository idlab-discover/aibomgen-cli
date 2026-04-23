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
		if strings.Contains(item, ":") {
			parts := strings.SplitN(item, ":", 2)
			name := strings.TrimSpace(parts[0])
			mitigation := ""
			if len(parts) > 1 {
				mitigation = strings.TrimSpace(parts[1])
			}
			if name != "" {
				ethics = append(ethics, cdx.MLModelCardEthicalConsideration{
					Name:               name,
					MitigationStrategy: mitigation,
				})
			}
		} else {
			ethics = append(ethics, cdx.MLModelCardEthicalConsideration{
				Name:               item,
				MitigationStrategy: "",
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
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			mt := strings.TrimSpace(parts[0])
			mv := strings.TrimSpace(parts[1])
			if mt != "" {
				metrics = append(metrics, cdx.MLPerformanceMetric{Type: mt, Value: mv})
			}
		} else if len(parts) == 1 {
			mt := strings.TrimSpace(parts[0])
			if mt != "" {
				metrics = append(metrics, cdx.MLPerformanceMetric{Type: mt, Value: ""})
			}
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
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if name != "" && val != "" {
				props = append(props, cdx.Property{Name: name, Value: val})
			}
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

		var role, orgName string
		if strings.Contains(pair, ":") {
			parts := strings.SplitN(pair, ":", 2)
			role = strings.ToLower(strings.TrimSpace(parts[0]))
			orgName = strings.TrimSpace(parts[1])
		} else {
			// No role specified, default to custodian.
			role = "custodian"
			orgName = strings.TrimSpace(pair)
		}

		if orgName == "" {
			continue
		}

		switch role {
		case "custodian", "custodians":
			governance.Custodians = &[]cdx.ComponentDataGovernanceResponsibleParty{{
				Organization: &cdx.OrganizationalEntity{Name: orgName},
			}}
			hasGovernance = true
		case "steward", "stewards", "curated", "curatedby":
			governance.Stewards = &[]cdx.ComponentDataGovernanceResponsibleParty{{
				Organization: &cdx.OrganizationalEntity{Name: orgName},
			}}
			hasGovernance = true
		case "owner", "owners", "funded", "fundedby":
			governance.Owners = &[]cdx.ComponentDataGovernanceResponsibleParty{{
				Organization: &cdx.OrganizationalEntity{Name: orgName},
			}}
			hasGovernance = true
		}
	}

	if !hasGovernance {
		return nil, fmt.Errorf("no valid governance roles found")
	}

	return governance, nil
}
