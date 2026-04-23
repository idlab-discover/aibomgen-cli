package builder

import (
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/metadata"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type BOMBuilder struct {
	Opts Options
}

func NewBOMBuilder(opts Options) *BOMBuilder {
	return &BOMBuilder{Opts: opts}
}

func (b BOMBuilder) Build(ctx BuildContext) (*cdx.BOM, error) {

	comp := buildMetadataComponent(ctx)

	bom := cdx.NewBOM()
	bom.Metadata = &cdx.Metadata{Component: comp}

	if err := AddMetaSerialNumber(bom); err != nil {
		return nil, err
	}
	if err := AddMetaTimestamp(bom); err != nil {
		return nil, err
	}
	if err := AddMetaTools(bom, "", GetAIBoMGenVersion()); err != nil {
		return nil, err
	}

	// Apply registry exactly once (no duplication).
	src := metadata.Source{
		ModelID:      strings.TrimSpace(ctx.ModelID),
		Scan:         ctx.Scan,
		HF:           ctx.HF,
		Readme:       ctx.Readme,
		SecurityTree: ctx.SecurityTree,
	}
	tgt := metadata.Target{
		BOM:                       bom,
		Component:                 comp,
		ModelCard:                 comp.ModelCard,
		IncludeEvidenceProperties: b.Opts.IncludeEvidenceProperties,
		HuggingFaceBaseURL:        b.Opts.HuggingFaceBaseURL,
	}

	for _, spec := range metadata.Registry() {
		metadata.ApplyFromSources(spec, src, tgt)
	}

	// Now properties, hashes and tags are populated — compute deterministic PURL and BOMRef.
	AddComponentPurl(comp)
	AddComponentBOMRef(comp)

	// Inject security scan findings as Component.Properties and BOM.Vulnerabilities.
	InjectSecurityData(bom, comp, ctx.SecurityTree, strings.TrimSpace(ctx.ModelID))

	return bom, nil
}

// BuildDataset builds a dataset component into BOM.components.
func (b BOMBuilder) BuildDataset(ctx DatasetBuildContext) (*cdx.Component, error) {

	comp := buildDatasetComponent(ctx)

	// Apply dataset registry.
	src := metadata.DatasetSource{
		DatasetID: strings.TrimSpace(ctx.DatasetID),
		Scan:      ctx.Scan,
		HF:        ctx.HF,
		Readme:    ctx.Readme,
	}
	tgt := metadata.DatasetTarget{
		Component:                 comp,
		IncludeEvidenceProperties: b.Opts.IncludeEvidenceProperties,
		HuggingFaceBaseURL:        b.Opts.HuggingFaceBaseURL,
	}

	for _, spec := range metadata.DatasetRegistry() {
		metadata.ApplyDatasetFromSources(spec, src, tgt)
	}

	AddComponentPurl(comp)
	AddComponentBOMRef(comp)
	return comp, nil
}

func buildMetadataComponent(ctx BuildContext) *cdx.Component {
	// Minimal skeleton; registry fills the rest.
	name := strings.TrimSpace(ctx.ModelID)
	if name == "" && strings.TrimSpace(ctx.Scan.Name) != "" {
		name = strings.TrimSpace(ctx.Scan.Name)
	}
	if name == "" {
		name = "model"
	}

	return &cdx.Component{
		Type:      cdx.ComponentTypeMachineLearningModel,
		Name:      name,
		ModelCard: &cdx.MLModelCard{},
	}
}

// buildDatasetComponent creates skeleton for DATASET component (DATA type).
func buildDatasetComponent(ctx DatasetBuildContext) *cdx.Component {
	name := strings.TrimSpace(ctx.DatasetID)
	if name == "" && strings.TrimSpace(ctx.Scan.Name) != "" {
		name = strings.TrimSpace(ctx.Scan.Name)
	}
	if name == "" {
		name = "dataset"
	}

	return &cdx.Component{
		Type: cdx.ComponentTypeData,
		Name: name,
	}
}
