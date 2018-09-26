package test

import "testing"

func TestVarsFromTFPlan(t *testing.T) {
	t.Parallel()

	name := "tf-test-vars-from-tfplan"

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	// Create TerraformPlan with metadata_key matching name
	tfplan := testMakeTF(t, tfSpecData{
		Kind:             TFKindPlan,
		Name:             name,
		ConfigMapSources: []string{name},
		TFVars: map[string]string{
			"metadata_key": name,
		},
	})
	t.Log(tfplan)
	testApply(t, namespace, tfplan)
	testWaitTF(t, TFKindPlan, namespace, name)
	defer testDelete(t, namespace, tfplan)

	// Create TerraformApply with tfvarFrom tfplan
	tfapply := testMakeTF(t, tfSpecData{
		Kind:             TFKindApply,
		Name:             name,
		ConfigMapSources: []string{name},
		TFVarsFrom: []TFSource{
			TFSource{
				TFPlan: name,
			},
		},
	})
	t.Log(tfapply)
	testApply(t, namespace, tfapply)
	testWaitTF(t, TFKindApply, namespace, name)
	defer testDelete(t, namespace, tfapply)

	// Verify output var matches varFrom value
	tf := testGetTF(t, TFKindApply, namespace, name)
	for _, output := range tf.Status.Outputs {
		if output.Name == "metadata_key" {
			assert(t, output.Value == name, "metadata_key value from TFVarFrom does not match")
			break
		}
	}

	// Run TerraformDestroy
	tfdestroy := testMakeTF(t, tfSpecData{
		Kind:             TFKindDestroy,
		Name:             name,
		ConfigMapSources: []string{name},
		TFVarsFrom: []TFSource{
			TFSource{
				TFApply: name,
			},
		},
	})
	t.Log(tfdestroy)
	testApply(t, namespace, tfdestroy)
	testWaitTF(t, TFKindDestroy, namespace, name)
	defer testDelete(t, namespace, tfdestroy)
}
