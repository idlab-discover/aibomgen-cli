package metadata

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

const contribLink = "[More Information Needed](https://github.com/huggingface/datasets/blob/master/CONTRIBUTING.md#how-to-contribute-to-the-dataset-cards)"

func TestRealText(t *testing.T) {
	for in, want := range map[string]string{
		"  Wiebe Vandendriessche ":     "Wiebe Vandendriessche",
		"[More Information Needed]":    "",
		contribLink:                    "",
		"[unknown](https://x.y)":       "",
		"[Acme](https://acme.example)": "[Acme](https://acme.example)",
	} {
		if got := realText(in); got != want {
			t.Errorf("realText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinkURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://arxiv.org/abs/1810.04805":          "https://arxiv.org/abs/1810.04805",
		"<https://arxiv.org/abs/1810.04805>":        "https://arxiv.org/abs/1810.04805",
		"[arXiv](https://arxiv.org/abs/1810.04805)": "https://arxiv.org/abs/1810.04805",
		"[https://example.com/]":                    "https://example.com/",
		"[paper](<https://example.com/p.pdf>)":      "https://example.com/p.pdf",
		contribLink:                                 "",
		"[More Information Needed]":                 "",
		"Coming soon":                               "",
		"https://a.b and more text":                 "",
		"":                                          "",
	} {
		if got := linkURL(in); got != want {
			t.Errorf("linkURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// README text is copied verbatim, template placeholders included: the BOM reflects
// what the model card says. Only entity names and URLs are filtered.
func TestDatasetReadmeTextVerbatim(t *testing.T) {
	comp := &cdx.Component{}
	src := DatasetSource{
		DatasetID: "org/ds",
		HF:        &fetcher.DatasetAPIResponse{ID: "org/ds"},
		Readme: &fetcher.DatasetReadmeCard{
			PaperURL:              contribLink,
			DemoURL:               "[https://example.com/]",
			PersonalSensitiveInfo: contribLink,
			DatasetCardContact:    "[More Information Needed]",
			DatasetDescription:    "... Description ...",
			CuratedBy:             "[More Information Needed]",
			FundedBy:              "Acme Foundation",
		},
	}
	for _, spec := range DatasetRegistry() {
		ApplyDatasetFromSources(spec, src, DatasetTarget{Component: comp})
	}

	// URL fields: placeholder link dropped, bracketed URL unwrapped.
	if comp.ExternalReferences == nil || len(*comp.ExternalReferences) != 2 || (*comp.ExternalReferences)[1].URL != "https://example.com/" {
		t.Fatalf("externalReferences = %+v, want website + demo", comp.ExternalReferences)
	}
	data := getComponentData(comp)
	if data == nil || data.Description != "... Description ..." {
		t.Fatalf("description = %+v", data)
	}
	if data.SensitiveData == nil || (*data.SensitiveData)[0] != "personal-info: "+contribLink {
		t.Fatalf("sensitiveData = %+v, want verbatim README text", data.SensitiveData)
	}
	if !hasProperty(comp, "huggingface:datasetContact") {
		t.Fatalf("README contact text must be kept verbatim")
	}
	// Entity names: placeholder organization dropped.
	if data.Governance == nil || data.Governance.Stewards != nil || data.Governance.Owners == nil {
		t.Fatalf("governance = %+v, want owners only", data.Governance)
	}
}

func TestModelCardReadmeTextVerbatim(t *testing.T) {
	comp := applyModel(Source{
		ModelID: "org/m",
		Readme: &fetcher.ModelReadmeCard{
			DevelopedBy:                "[More Information Needed]",
			PaperURL:                   "[https://example.com/]",
			DirectUse:                  "[More Information Needed]",
			BiasRisksLimitations:       "... Risks ...",
			ModelCardContact:           "[More Information Needed]",
			EnvironmentalHardwareType:  "A potato",
			EnvironmentalCarbonEmitted: "[More Information Needed]",
		},
	})
	mc := comp.ModelCard
	if mc.Considerations == nil || mc.Considerations.UseCases == nil || (*mc.Considerations.UseCases)[0] != "[More Information Needed]" {
		t.Fatalf("useCases = %+v, want verbatim README text", mc.Considerations)
	}
	if lim := mc.Considerations.TechnicalLimitations; lim == nil || (*lim)[0] != "... Risks ..." {
		t.Fatalf("technicalLimitations = %+v", lim)
	}
	env := mc.Considerations.EnvironmentalConsiderations
	if env == nil || env.Properties == nil || len(*env.Properties) != 2 {
		t.Fatalf("environmental = %+v, want both values verbatim", env)
	}
	if !hasProperty(comp, "huggingface:modelCardContact") {
		t.Fatalf("README contact text must be kept verbatim")
	}
	if refs := comp.ExternalReferences; refs == nil || len(*refs) != 2 || (*refs)[1].URL != "https://example.com/" {
		t.Fatalf("externalReferences = %+v", refs)
	}
	// Entity names: a placeholder "Developed by" doesn't become an author or manufacturer.
	if got := authorNames(comp); len(got) != 1 || got[0] != "org" {
		t.Fatalf("authors = %v, want namespace fallback", got)
	}
	if comp.Manufacturer != nil {
		t.Fatalf("manufacturer = %+v, want none", comp.Manufacturer)
	}
}

func TestModelCardEthicalConsiderationsName(t *testing.T) {
	ethics := func(r *fetcher.ModelReadmeCard) []cdx.MLModelCardEthicalConsideration {
		comp := applyModel(Source{ModelID: "org/m", Readme: r})
		c := comp.ModelCard.Considerations
		if c == nil || c.EthicalConsiderations == nil {
			return nil
		}
		return *c.EthicalConsiderations
	}

	got := ethics(&fetcher.ModelReadmeCard{EthicalConsiderations: "Ethics text.", BiasRisksLimitations: "Limits.", BiasRecommendations: "Be careful."})
	if len(got) != 1 || got[0].Name != "Ethics text." || got[0].MitigationStrategy != "Be careful." {
		t.Fatalf("dedicated section: %+v", got)
	}
	got = ethics(&fetcher.ModelReadmeCard{BiasRisksLimitations: "Limits."})
	if len(got) != 1 || got[0].Name != "Limits." {
		t.Fatalf("limitations fallback: %+v", got)
	}
	got = ethics(&fetcher.ModelReadmeCard{BiasRecommendations: "Be careful."})
	if len(got) != 1 || got[0].Name != "bias_risks_limitations" {
		t.Fatalf("recommendations only: %+v", got)
	}
}

func TestModelDescriptionSources(t *testing.T) {
	desc := func(src Source) string {
		src.ModelID = "org/m"
		return applyModel(src).Description
	}
	full := &fetcher.ModelReadmeCard{Summary: "From front matter.", DescriptionSection: "From a section.", LeadParagraph: "From the lead."}
	if got := desc(Source{Readme: full}); got != "From front matter." {
		t.Fatalf("summary first: %q", got)
	}
	if got := desc(Source{Readme: &fetcher.ModelReadmeCard{Summary: "[More Information Needed]", DescriptionSection: "From a section.", LeadParagraph: "From the lead."}}); got != "From a section." {
		t.Fatalf("placeholder summary should fall through: %q", got)
	}
	if got := desc(Source{Readme: &fetcher.ModelReadmeCard{LeadParagraph: "From the lead."}}); got != "From the lead." {
		t.Fatalf("lead paragraph last: %q", got)
	}
	api := &fetcher.ModelAPIResponse{CardData: map[string]any{"summary": "  From\n the API.  "}}
	if got := desc(Source{HF: api, Readme: &fetcher.ModelReadmeCard{DescriptionSection: "From a section."}}); got != "From the API." {
		t.Fatalf("cardData before README body: %q", got)
	}
	if got := desc(Source{Readme: &fetcher.ModelReadmeCard{}}); got != "" {
		t.Fatalf("want no description, got %q", got)
	}
}
