package test

import "testing"

// TestApplyTFPlan runs a tfplan then a tfapply that uses the planfile on GCS.
func TestApplyTFPlan(t *testing.T) {
	t.Parallel()

	name := "tf-test-apply-plan"

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
		ConditionReady,
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
	testConfigMapSourceTF(t, TFKindDestroy, name, name, true)
}
