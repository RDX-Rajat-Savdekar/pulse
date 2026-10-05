package manifest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKubernetesManifestsCoverTheRuntime(t *testing.T) {
	kinds := map[string]int{}
	names := map[string]map[string]bool{}
	err := filepath.WalkDir("k8s", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, "kustomization.yaml") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, doc := range bytes.Split(body, []byte("\n---")) {
			doc = bytes.TrimSpace(doc)
			if len(doc) == 0 {
				continue
			}
			var obj struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
			}
			if err := yaml.Unmarshal(doc, &obj); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if obj.Kind == "" || obj.Metadata.Name == "" {
				t.Fatalf("%s: missing kind or name", path)
			}
			kinds[obj.Kind]++
			if names[obj.Kind] == nil {
				names[obj.Kind] = map[string]bool{}
			}
			names[obj.Kind][obj.Metadata.Name] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"Deployment", "Service", "HorizontalPodAutoscaler", "ConfigMap", "Secret"} {
		if kinds[kind] == 0 {
			t.Fatalf("missing %s", kind)
		}
	}
	for _, name := range []string{"ingest", "processor", "query", "graphql", "kafka", "postgres", "redis", "prometheus", "grafana"} {
		if !names["Deployment"][name] {
			t.Fatalf("missing deployment %s", name)
		}
		if !names["Service"][name] {
			t.Fatalf("missing service %s", name)
		}
	}
	if !names["HorizontalPodAutoscaler"]["ingest"] || !names["HorizontalPodAutoscaler"]["processor"] {
		t.Fatal("expected HPAs for ingest and processor")
	}
	if !names["ConfigMap"]["pulse-config"] {
		t.Fatal("missing pulse-config")
	}

	kustomize, err := os.ReadFile("kustomization.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"namespace.yaml", "configmap.yaml", "secret.yaml", "hpa.yaml", "ingest.yaml"} {
		if !bytes.Contains(kustomize, []byte(file)) {
			t.Fatalf("kustomization does not list %s", file)
		}
	}

	ci, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(ci, []byte("go test ./...")) || !bytes.Contains(ci, []byte("kubectl kustomize deploy")) {
		t.Fatal("ci workflow is missing test or manifest validation")
	}

	prom, err := os.ReadFile("prometheus/prometheus.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"ingest:8080", "processor:9102", "query:9103", "graphql:8081"} {
		if !bytes.Contains(prom, []byte(target)) {
			t.Fatalf("prometheus missing %s", target)
		}
	}
	dash, err := os.ReadFile("grafana/provisioning/dashboards/pulse.json")
	if err != nil {
		t.Fatal(err)
	}
	var dashboard map[string]any
	if err := json.Unmarshal(dash, &dashboard); err != nil {
		t.Fatal(err)
	}
	if dashboard["title"] != "Pulse" {
		t.Fatalf("dashboard title %v", dashboard["title"])
	}
}
