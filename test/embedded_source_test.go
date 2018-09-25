package test

import (
	"testing"
)

func testEmbeddedSourceTF(t *testing.T, kind TFKind, name string) {
	embeddedSpec := helperLoadBytes(t, defaultTFSourcePath)
	tf := testMakeTF(t, tfSpecData{
		Kind:            kind,
		Name:            name,
		EmbeddedSources: []string{string(embeddedSpec)},
		TFVarsMap: map[string]string{
			"metadata_key": name,
			"region":       "us-west1",
		},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, kind, namespace, name)
	defer testDelete(t, namespace, tf)
}

// TestEmbeddedSource runs a plan,apply,destroy in serial with an embedded terraform source.
func TestEmbeddedSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-src"
	testEmbeddedSourceTF(t, TFKindPlan, name)
	testEmbeddedSourceTF(t, TFKindApply, name)
	testEmbeddedSourceTF(t, TFKindDestroy, name)
}

func TestEmbeddedSourceFromTFPlanAndTFApply(t *testing.T) {
	t.Parallel()

	name := "tf-test-src-tfplan-or-tfapply"
	testEmbeddedSourceTF(t, TFKindPlan, name)
	testEmbeddedSourceTF(t, TFKindApply, name)

	// Test with both tfapply and tfplan present.
	// Create tfdestroy
	tfdestroy := testMakeTF(t, tfSpecData{
		Kind: TFKindDestroy,
		Name: name,
		TFSources: []map[string]string{
			map[string]string{
				"tfplan":  name,
				"tfapply": name,
			},
		},
	})
	t.Log(tfdestroy)
	testApply(t, namespace, tfdestroy)
	testWaitTF(t, TFKindDestroy, namespace, name)
	defer testDelete(t, namespace, tfdestroy)

}
