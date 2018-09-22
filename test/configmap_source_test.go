package test

import (
	"testing"
)

func testConfigMapSourceTF(t *testing.T, kind TFKind, name, cmName string) {
	tf := testMakeTF(t, tfSpecData{
		Kind:             kind,
		Name:             name,
		ConfigMapSources: []string{cmName},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, kind, namespace, name)
	defer testDelete(t, namespace, tf)
}

// TestConfigMapSource runs a plan,apply,destroy in sequence using a configmap source.
func TestConfigMapSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-cm"
	cmName := "tf-test-cm"
	testApplyTFSourceConfigMap(t, namespace, cmName)
	defer testDeleteTFSourceConfigMap(t, namespace, cmName)

	testConfigMapSourceTF(t, TFKindPlan, name, cmName)
	testConfigMapSourceTF(t, TFKindApply, name, cmName)
	testConfigMapSourceTF(t, TFKindDestroy, name, cmName)
}

// TestConfigMapSourceApplyPlan runs a plan then an apply that uses the planfile on GCS.
func TestConfigMapSourceApplyPlan(t *testing.T) {
	t.Parallel()

	name := "tf-test-cm-apply-plan"
	cmName := "tf-test-cm-apply-plan"
	testApplyTFSourceConfigMap(t, namespace, cmName)
	defer testDeleteTFSourceConfigMap(t, namespace, cmName)

	// Create tfplan
	tfplan := testMakeTF(t, tfSpecData{
		Kind:             TFKindPlan,
		Name:             name,
		ConfigMapSources: []string{cmName},
	})
	t.Log(tfplan)
	testApply(t, namespace, tfplan)
	testWaitTF(t, TFKindPlan, namespace, name)
	defer testDelete(t, namespace, tfplan)

	// Create tfapply
	tfapply := testMakeTF(t, tfSpecData{
		Kind:             TFKindApply,
		Name:             name,
		ConfigMapSources: []string{cmName},
		TFPlan:           name,
	})
	t.Log(tfapply)
	testApply(t, namespace, tfapply)
	testWaitTF(t, TFKindApply, namespace, name)
	defer testDelete(t, namespace, tfapply)

	// Create tfdestroy
	testConfigMapSourceTF(t, TFKindDestroy, name, cmName)
}
