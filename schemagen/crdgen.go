package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kubeyaml "sigs.k8s.io/yaml"
)

const chartMajorVersion = "1"

const crdKindAnnotation = "crd.bundle.appcat.io/kind"
const crdListKindAnnotation = "crd.bundle.appcat.io/listKind"
const crdSingularAnnotation = "crd.bundle.appcat.io/singular"
const crdPluralAnnotation = "crd.bundle.appcat.io/plural"

type chart struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run crdgen.go <chart_directory>")
		os.Exit(1)
	}
	chartDir := os.Args[1]

	schema, err := valuesSchema(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		panic(err)
	}
	var chart chart
	chartFile := filepath.Join(chartDir, "Chart.yaml")
	data, err := os.ReadFile(chartFile)
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		panic(err)
	}

	var crd apiextv1.CustomResourceDefinition
	crd.SetGroupVersionKind(apiextv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	names, err := names(chart)
	if err != nil {
		panic(err)
	}
	crd.Spec.Names = names

	group := fmt.Sprintf("v%s.%s.bundles.appcat.io", chartMajorVersion, chart.Name)
	crd.Name = fmt.Sprintf("%s.%s", names.Plural, group)
	crd.Spec.Group = group
	crd.Spec.Scope = apiextv1.NamespaceScoped
	crd.Spec.Versions = []apiextv1.CustomResourceDefinitionVersion{
		{
			Name:    "bundle",
			Served:  true,
			Storage: true,
			Schema: &apiextv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextv1.JSONSchemaProps{
						"spec": {
							Type:        "object",
							Description: "Configures the desired state of the service.",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"version": {
									Type:        "string",
									Description: "The version of the service. Every change to this field together with the `.spec.values` field creates a new revision of the service.",
								},
								"values": {
									Type:        "object",
									Description: "This field together with the `.spec.version` field defines the configuration of the service. Every change to either of these two fields creates a new revision of the service.",
									Properties:  schema.Properties,
								},
								"desiredRevision": {
									Type:        "string",
									Description: "The desired revision of the service.",
								},
							},
						},
					},
				},
			},
		},
	}

	yamlData, err := kubeyaml.Marshal(crd)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(yamlData))
}

func names(chart chart) (apiextv1.CustomResourceDefinitionNames, error) {
	kind := chart.Annotations[crdKindAnnotation]
	if kind == "" {
		kind = "Instance"
	}
	plural := chart.Annotations[crdPluralAnnotation]
	if plural == "" {
		plural = strings.ToLower(kind) + "s"
	}

	listKind := chart.Annotations[crdListKindAnnotation]
	singular := chart.Annotations[crdSingularAnnotation]

	return apiextv1.CustomResourceDefinitionNames{
		Kind:     kind,
		ListKind: listKind,
		Plural:   plural,
		Singular: singular,
	}, nil
}

func valuesSchema(valuesFile string) (apiextv1.JSONSchemaProps, error) {
	var node yaml.Node
	data, err := os.ReadFile(valuesFile)
	if err != nil {
		return apiextv1.JSONSchemaProps{}, err
	}
	if err := yaml.Unmarshal(data, &node); err != nil {
		return apiextv1.JSONSchemaProps{}, err
	}

	if len(node.Content) == 0 {
		return apiextv1.JSONSchemaProps{}, fmt.Errorf("empty YAML document")
	}
	if len(node.Content) > 1 {
		return apiextv1.JSONSchemaProps{}, fmt.Errorf("multiple YAML documents found")
	}
	top := node.Content[0] // Unwrap the document node
	if top.Kind != yaml.MappingNode {
		return apiextv1.JSONSchemaProps{}, fmt.Errorf("top-level YAML node is not a mapping")
	}

	// Convert the YAML node to JSON schema properties
	schemaProps, err := convertYAMLNodeToJSONSchema(top, "")
	if err != nil {
		return apiextv1.JSONSchemaProps{}, err
	}
	return schemaProps, nil
}

func convertYAMLNodeToJSONSchema(node *yaml.Node, path string) (apiextv1.JSONSchemaProps, error) {
	if node == nil {
		return apiextv1.JSONSchemaProps{Type: "object"}, nil
	}

	switch node.Kind {
	case yaml.AliasNode:
		return convertYAMLNodeToJSONSchema(node.Alias, path)

	case yaml.MappingNode:
		props := make(map[string]apiextv1.JSONSchemaProps)
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if keyNode == nil {
				continue
			}
			valueSchema, err := convertYAMLNodeToJSONSchema(valueNode, path+"."+keyNode.Value)
			if err != nil {
				return apiextv1.JSONSchemaProps{}, fmt.Errorf("at %s: %s", path, err)
			}
			valueSchema.Description = stripComment(keyNode.HeadComment)
			props[keyNode.Value] = valueSchema
		}

		return apiextv1.JSONSchemaProps{
			Type:       "object",
			Properties: props,
		}, nil

	case yaml.SequenceNode:
		items := apiextv1.JSONSchemaProps{Type: "string"}
		if len(node.Content) > 0 {
			var err error
			items, err = convertYAMLNodeToJSONSchema(node.Content[0], path+"[0]")
			if err != nil {
				return apiextv1.JSONSchemaProps{}, fmt.Errorf("at %s: %s", path, err)
			}
		}

		return apiextv1.JSONSchemaProps{
			Type: "array",
			Items: &apiextv1.JSONSchemaPropsOrArray{
				Schema: &items,
			},
		}, nil

	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			return apiextv1.JSONSchemaProps{Nullable: true}, nil
		}

		var schemaType string
		switch node.Tag {
		case "!!bool":
			schemaType = "boolean"
		case "!!int":
			schemaType = "integer"
		case "!!float":
			schemaType = "number"
		case "!!str":
			schemaType = "string"
		default:
			return apiextv1.JSONSchemaProps{}, fmt.Errorf("unsupported YAML scalar tag: %s", node.Tag)
		}

		return apiextv1.JSONSchemaProps{
			Type: schemaType,
		}, nil
	default:
		return apiextv1.JSONSchemaProps{}, fmt.Errorf("unsupported YAML node kind: %v", node.Kind)
	}
}

func stripComment(s string) string {
	lines := strings.Split(s, "\n")
	strippedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		strippedLine := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		if strippedLine != "" {
			strippedLines = append(strippedLines, strippedLine)
		}
	}
	return strings.Join(strippedLines, "\n")
}
