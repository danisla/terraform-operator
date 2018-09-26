package test

import (
	"testing"
)

func testConfigMapSourceTF(t *testing.T, kind TFKind, name, cmName string) {
	spec := testMakeTF(t, tfSpecData{
		Kind:             kind,
		Name:             name,
		ConfigMapSources: []string{cmName},
		TFVars: map[string]string{
			"metadata_key": name,
		},
	})
	defer testDelete(t, namespace, spec)

	t.Log(spec)
	testApply(t, namespace, spec)
	tf := testWaitTF(t, kind, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionPodComplete,
		ConditionProviderConfigReady,
		ConditionSourceReady,
	})
}

// TestConfigMapSource runs a plan,apply,destroy in sequence using a configmap source.
func TestConfigMapSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-cm"

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	testConfigMapSourceTF(t, TFKindPlan, name, name)
	testConfigMapSourceTF(t, TFKindApply, name, name)
	testConfigMapSourceTF(t, TFKindDestroy, name, name)
}

// TestConfigMapSourceApplyPlan runs a plan then an apply that uses the planfile on GCS.
func TestConfigMapSourceApplyPlan(t *testing.T) {
	t.Parallel()

	name := "tf-test-cm-apply-plan"

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	// Create tfplan
	tfplan := testMakeTF(t, tfSpecData{
		Kind:             TFKindPlan,
		Name:             name,
		ConfigMapSources: []string{name},
	})
	defer testDelete(t, namespace, tfplan)
	t.Log(tfplan)
	testApply(t, namespace, tfplan)
	tf := testWaitTF(t, TFKindPlan, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionPodComplete,
		ConditionProviderConfigReady,
		ConditionSourceReady,
	})

	// Create tfapply
	tfapply := testMakeTF(t, tfSpecData{
		Kind:             TFKindApply,
		Name:             name,
		ConfigMapSources: []string{name},
		TFPlan:           name,
	})
	t.Log(tfapply)
	testApply(t, namespace, tfapply)
	testWaitTF(t, TFKindApply, namespace, name)
	defer testDelete(t, namespace, tfapply)

	// Create tfdestroy
	testConfigMapSourceTF(t, TFKindDestroy, name, name)
}
