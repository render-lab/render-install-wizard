package manifest

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestSampleMatchesSchema(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../manifest/manifest.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	sampleBytes, err := os.ReadFile("../../manifest/manifest.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("manifest.schema.json", schemaDoc); err != nil {
		t.Fatalf("add resource: %v", err)
	}
	sch, err := c.Compile("manifest.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(sampleBytes))
	if err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("sample does not conform to schema: %v", err)
	}
}

func TestParseSample(t *testing.T) {
	sampleBytes, err := os.ReadFile("../../manifest/manifest.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	m, err := Parse(sampleBytes)
	if err != nil {
		t.Fatalf("parse sample: %v", err)
	}
	if m.Version != "1" {
		t.Errorf("Version = %q, want %q", m.Version, "1")
	}
	if len(m.Components) != 3 {
		t.Errorf("len(Components) = %d, want 3", len(m.Components))
	}
	if len(m.Tools) != 4 {
		t.Errorf("len(Tools) = %d, want 4", len(m.Tools))
	}
}

func TestParseRejectsWrongVersion(t *testing.T) {
	sampleBytes, err := os.ReadFile("../../manifest/manifest.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(sampleBytes, &raw); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	raw["version"] = "2"
	modified, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal modified: %v", err)
	}
	if _, err := Parse(modified); err == nil {
		t.Fatal("Parse accepted a manifest with version \"2\", want error")
	}
}
