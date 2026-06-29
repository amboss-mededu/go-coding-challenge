package challenge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/amboss-mededu/go-coding-challenge/model"
)

// TestMain runs the generator before any test in this package executes.
func TestMain(m *testing.M) {
	cmd := exec.Command("go", "run", "./cmd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generator failed: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestSchemaFidelityGaps documents the gap between "tests pass" and
// "schema is fully enforced at runtime". Each subtest shows current Go
// behaviour; the "schema says" comment describes what a fully faithful
// implementation would do. Tests break automatically if the generator is
// later enhanced to enforce the constraint.
func TestSchemaFidelityGaps(t *testing.T) {
	t.Run("product should have required id", func(t *testing.T) {
		var p model.Product
		err := json.Unmarshal([]byte(`{"name":"x","price":1.0}`), &p)
		if err != nil {
			t.Fatalf("gap closed: expected no error for missing required field, got: %v", err)
		}
		// gap: Id is "" instead of an error
		if p.Id != "" {
			t.Fatalf("expected empty Id (gap), got %q", p.Id)
		}
	})

	t.Run("product should have required price", func(t *testing.T) {
		// schema says: required field "price" must be present — absence is an error
		var p model.Product
		err := json.Unmarshal([]byte(`{"id":"1","name":"x"}`), &p)
		if err != nil {
			t.Fatalf("gap closed: expected no error for missing required field, got: %v", err)
		}
		if p.Price != 0 {
			t.Fatalf("expected zero Price (gap), got %v", p.Price)
		}
	})

	t.Run("article should have required title", func(t *testing.T) {
		// schema says: required field "title" must be present — absence is an error
		var a model.Article
		err := json.Unmarshal([]byte(`{"id":"1"}`), &a)
		if err != nil {
			t.Fatalf("gap closed: expected no error for missing required field, got: %v", err)
		}
		// gap: Title is "" instead of an error
		if a.Title != "" {
			t.Fatalf("expected empty Title (gap), got %q", a.Title)
		}
	})

	t.Run("category should reject invalid enum kind", func(t *testing.T) {
		// schema says: kind must be one of basic-science|basic_science|clinical|other
		var c model.Category
		if err := json.Unmarshal([]byte(`{"kind":"bogus"}`), &c); err == nil {
			t.Fatalf("expected error for invalid enum value, got none (Kind=%q)", c.Kind)
		}

		// a valid enum value must still be accepted
		if err := json.Unmarshal([]byte(`{"kind":"clinical"}`), &c); err != nil {
			t.Fatalf("expected valid enum value to be accepted, got: %v", err)
		}
		if c.Kind != model.CategoryKindClinical {
			t.Fatalf("expected Kind=clinical, got %q", c.Kind)
		}
	})

	t.Run("product should reject invalid enum classification", func(t *testing.T) {
		// schema says: classification must be one of basic_science|basic-science|applied|experimental
		var p model.Product
		if err := json.Unmarshal([]byte(`{"id":"1","name":"x","price":1.0,"classification":"unknown"}`), &p); err == nil {
			t.Fatalf("expected error for invalid enum value, got none (Classification=%q)", p.Classification)
		}

		// a valid enum value must still be accepted
		if err := json.Unmarshal([]byte(`{"id":"1","name":"x","price":1.0,"classification":"applied"}`), &p); err != nil {
			t.Fatalf("expected valid enum value to be accepted, got: %v", err)
		}
		if p.Classification != model.ProductClassificationApplied {
			t.Fatalf("expected Classification=applied, got %q", p.Classification)
		}
	})
}
