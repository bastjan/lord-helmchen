package main

import (
	"bytes"
	"fmt"
	"os"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/registry"
)

func main() {
	c, err := registry.NewClient()
	if err != nil {
		panic(err)
	}
	tags, err := c.Tags("ghcr.io/stefanprodan/charts/podinfo")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "tags:", tags)
	res, err := c.Pull("oci://ghcr.io/stefanprodan/charts/podinfo@sha256:476bed61733536f99e7331b0fe4cc9fd70bc6497a855ad38ba49b72de50c1132", registry.PullOptWithChart(true))
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stderr, "ref:", res.Ref)

	chart, err := loader.LoadArchive(bytes.NewReader(res.Chart.Data))
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "annotations:", chart.Metadata.Annotations)

	var valuesYaml []byte
	for _, f := range chart.Raw {
		if f.Name == "values.yaml" {
			valuesYaml = f.Data
			break
		}
	}
	if valuesYaml == nil {
		panic("values.yaml not found")
	}
	fmt.Println(string(valuesYaml))
}
