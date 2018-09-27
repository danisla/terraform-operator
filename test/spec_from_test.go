package test

import (
	"testing"
)

// TestSpecFromTFPlan verifies that a spec can be referenced from an existing TerraformPlan.
func TestSpecFrom(t *testing.T) {
	t.Parallel()

	name := "tf-test-specfromplan"

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	// Create the tfplan
	tfplan := testMakeTF(t, tfSpecData{
		Kind:             TFKindPlan,
		Name:             name,
		ConfigMapSources: []string{name},
		TFVars: map[string]string{
			"metadata_key": name,
		},
	})
	defer testDelete(t, namespace, tfplan)
	t.Log(tfplan)
	testApply(t, namespace, tfplan)

	// create a tfapply that uses the spec from the tfplan
	// create in parallel to verify it waits properly.
	tfapply := testMakeTFSpecFrom(t, tfSpecFromData{
		Kind:         TFKindApply,
		Name:         name,
		TFPlan:       name,
		WaitForReady: true,
	})
	defer testDelete(t, namespace, tfapply)
	t.Log(tfapply)
	testApply(t, namespace, tfapply)

	// create a tfdestroy that uses the spec from the tfapply
	// create in parallel to verify it waits properly.
	tfdestroy := testMakeTFSpecFrom(t, tfSpecFromData{
		Kind:         TFKindDestroy,
		Name:         name,
		TFApply:      name,
		WaitForReady: true,
	})
	defer testDelete(t, namespace, tfdestroy)
	t.Log(tfdestroy)
	testApply(t, namespace, tfdestroy)

	// Wait for the tfplan
	tf := testWaitTF(t, TFKindPlan, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionPodComplete,
		ConditionReady,
	})

	// Wait for the tfapply
	tf = testWaitTF(t, TFKindApply, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionSpecFromReady,
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionPodComplete,
		ConditionReady,
	})

	// Wait for the tfdestroy
	tf = testWaitTF(t, TFKindDestroy, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionSpecFromReady,
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionPodComplete,
		ConditionReady,
	})
}
