package validator

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Official CycloneDX JSON schemas, from CycloneDX/specification@1ce97b2
// (schema/). The spdx, jsf and cryptography-defs schemas are referenced by them.
//
//go:embed schema/*.json
var schemaFS embed.FS

const schemaBaseURL = "http://cyclonedx.org/schema/"

// embeddedLoader serves http://cyclonedx.org/schema/* from schemaFS, so schema
// validation never touches the network.
type embeddedLoader struct{}

func (embeddedLoader) Load(url string) (any, error) {
	name, ok := strings.CutPrefix(url, schemaBaseURL)
	if !ok {
		return nil, fmt.Errorf("schema %s is not embedded", url)
	}
	data, err := schemaFS.ReadFile("schema/" + name)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(data))
}

var printer = message.NewPrinter(language.English)

// ValidateJSONSchema validates a JSON-encoded BOM against the official CycloneDX
// JSON schema for its specVersion (1.2 to 1.7), asserting formats such as
// date-time and iri-reference. It returns one message per violation.
// skipped is non-empty when no schema applies (e.g. spec 1.0/1.1, which only
// have an XML schema); a missing or unknown specVersion is itself a violation.
func ValidateJSONSchema(data []byte) (violations []string, skipped string) {
	var head struct {
		SpecVersion string `json:"specVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %v", err)}, ""
	}

	switch head.SpecVersion {
	case "1.0", "1.1":
		return nil, fmt.Sprintf("CycloneDX %s has no JSON schema; schema validation skipped", head.SpecVersion)
	case "1.2", "1.3", "1.4", "1.5", "1.6", "1.7":
	default:
		return []string{fmt.Sprintf("schema: unsupported or missing specVersion %q", head.SpecVersion)}, ""
	}

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(embeddedLoader{})
	// ponytail: compiled per call (~tens of ms); cache per version if validate ever processes many files.
	sch, err := c.Compile(schemaBaseURL + "bom-" + head.SpecVersion + ".schema.json")
	if err != nil {
		return []string{fmt.Sprintf("schema: cannot compile CycloneDX %s schema: %v", head.SpecVersion, err)}, ""
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return []string{fmt.Sprintf("invalid JSON: %v", err)}, ""
	}

	verr, ok := sch.Validate(doc).(*jsonschema.ValidationError)
	if !ok {
		return nil, ""
	}
	collectLeaves(verr, &violations)
	return violations, ""
}

// collectLeaves flattens a validation error tree into "schema: /path: message" lines.
func collectLeaves(e *jsonschema.ValidationError, out *[]string) {
	if len(e.Causes) == 0 {
		*out = append(*out, fmt.Sprintf("schema: /%s: %s", strings.Join(e.InstanceLocation, "/"), e.ErrorKind.LocalizedString(printer)))
		return
	}
	for _, c := range e.Causes {
		collectLeaves(c, out)
	}
}
