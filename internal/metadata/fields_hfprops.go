package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func hfPropFields() []FieldSpec {
	return []FieldSpec{
		hfProp(ComponentPropertiesHuggingFaceLastModified, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil {
				return nil, false
			}
			s := strings.TrimSpace(r.LastMod)
			return s, s != ""
		}),
		hfProp(ComponentPropertiesHuggingFaceCreatedAt, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil {
				return nil, false
			}
			s := strings.TrimSpace(r.CreatedAt)
			return s, s != ""
		}),
		hfProp(ComponentPropertiesHuggingFaceLanguage, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil {
				return nil, false
			}
			s := extractLanguage(r.CardData)
			return s, s != ""
		}),
		hfProp(ComponentPropertiesHuggingFaceUsedStorage, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil || r.UsedStorage <= 0 {
				return nil, false
			}
			return r.UsedStorage, true
		}),
		hfProp(ComponentPropertiesHuggingFacePrivate, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil {
				return nil, false
			}
			return r.Private, true
		}),
		hfProp(ComponentPropertiesHuggingFaceLibraryName, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil {
				return nil, false
			}
			s := strings.TrimSpace(r.LibraryName)
			return s, s != ""
		}),
		hfProp(ComponentPropertiesHuggingFaceDownloads, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil || r.Downloads <= 0 {
				return nil, false
			}
			return r.Downloads, true
		}),
		hfProp(ComponentPropertiesHuggingFaceLikes, 0.2, func(src Source) (any, bool) {
			r := src.HF
			if r == nil || r.Likes <= 0 {
				return nil, false
			}
			return r.Likes, true
		}),
		hfProp(ComponentPropertiesHuggingFaceBaseModel, 0.2, func(src Source) (any, bool) {
			r := src.Readme
			if r == nil {
				return nil, false
			}
			s := strings.TrimSpace(r.BaseModel)
			return s, s != ""
		}),
		hfProp(ComponentPropertiesHuggingFaceContact, 0.2, func(src Source) (any, bool) {
			r := src.Readme
			if r == nil {
				return nil, false
			}
			s := strings.TrimSpace(r.ModelCardContact)
			return s, s != ""
		}),
	}
}

func hfProp(key Key, weight float64, get func(src Source) (any, bool)) FieldSpec {
	return FieldSpec{
		Key:      key,
		Weight:   weight,
		Required: false,
		Sources: []func(Source) (any, bool){
			func(src Source) (any, bool) {
				if get == nil {
					return nil, false
				}
				return get(src)
			},
		},
		Parse: func(value string) (any, error) {
			return parseNonEmptyString(value, "property")
		},
		Apply: func(tgt Target, value any) error {
			input, ok := value.(applyInput)
			if !ok {
				return fmt.Errorf("invalid input for %s", key)
			}
			if tgt.Component == nil {
				return fmt.Errorf("component is nil")
			}
			v := input.Value
			propName := strings.TrimPrefix(key.String(), "BOM.metadata.component.properties.")
			setProperty(tgt.Component, propName, strings.TrimSpace(fmt.Sprint(v)))
			return nil
		},
		Present: func(b *cdx.BOM) bool {
			c := bomComponent(b)
			propName := strings.TrimPrefix(key.String(), "BOM.metadata.component.properties.")
			ok := c != nil && hasProperty(c, propName)
			return ok
		},
	}
}
