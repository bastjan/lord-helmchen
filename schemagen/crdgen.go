package main

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kubeyaml "sigs.k8s.io/yaml"
)

const chartMajorVersion = "1"
const chartName = "vshnjuiceshop"

const kind = "JuiceShop"
const listKind = "JuiceShopList"
const plural = "juiceshops"
const singular = "juiceshop"

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run crdgen.go <values.yaml>")
		os.Exit(1)
	}
	valuesFile := os.Args[1]

	schema, err := schema(valuesFile)
	if err != nil {
		panic(err)
	}

	var crd apiextv1.CustomResourceDefinition
	crd.SetGroupVersionKind(apiextv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	group := fmt.Sprintf("v%s.%s.bundles.appcat.io", chartMajorVersion, chartName)
	crd.Spec.Names.Kind = kind
	crd.Spec.Names.ListKind = listKind
	crd.Spec.Names.Plural = plural
	crd.Spec.Names.Singular = singular

	crd.Name = fmt.Sprintf("%s.%s", plural, group)
	crd.Spec.Group = group
	crd.Spec.Versions = []apiextv1.CustomResourceDefinitionVersion{
		{
			Name:    "bundle",
			Served:  true,
			Storage: true,
			Schema: &apiextv1.CustomResourceValidation{
				OpenAPIV3Schema: &schema,
			},
		},
	}
	crd.Spec.Scope = apiextv1.NamespaceScoped

	yamlData, err := kubeyaml.Marshal(crd)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(yamlData))
}

func schema(valuesFile string) (apiextv1.JSONSchemaProps, error) {
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
			valueSchema.Description = keyNode.HeadComment
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
