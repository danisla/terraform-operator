package test

import (
	"testing"
)

func testEmbeddedSourceTF(t *testing.T, kind TFKind, name string, delete bool) string {
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
	if delete {
		defer testDelete(t, namespace, tf)
	}
	testWaitTF(t, kind, namespace, name)
	return tf
}

// TestEmbeddedSource runs a plan,apply,destroy in serial with an embedded terraform source.
func TestEmbeddedSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-src"
	testEmbeddedSourceTF(t, TFKindPlan, name, true)

	tf := testEmbeddedSourceTF(t, TFKindApply, name, false)
	testVerifyOutputVars(t, namespace, name)
	testDelete(t, namespace, tf)

	testEmbeddedSourceTF(t, TFKindDestroy, name, true)
}

func TestEmbeddedSourceFromTFPlanAndTFApply(t *testing.T) {
	t.Parallel()

	name := "tf-test-src-tfplan-or-tfapply"

	tfplan := testEmbeddedSourceTF(t, TFKindPlan, name, false)
	defer testDelete(t, namespace, tfplan)

	tfapply := testEmbeddedSourceTF(t, TFKindApply, name, false)
	testVerifyOutputVars(t, namespace, name)
	defer testDelete(t, namespace, tfapply)

	// Test with both tfapply and tfplan present.
	// Create tfdestroy
	tfdestroy := testMakeTF(t, tfSpecData{
		Kind: TFKindDestroy,
		Name: name,
		TFSources: []TFSource{
			TFSource{
				TFPlan:  name,
				TFApply: name,
			},
		},
	})
	t.Log(tfdestroy)
	testApply(t, namespace, tfdestroy)
	testWaitTF(t, TFKindDestroy, namespace, name)
	testVerifyOutputVars(t, namespace, name)
	defer testDelete(t, namespace, tfdestroy)
}
