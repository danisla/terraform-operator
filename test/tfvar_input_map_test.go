package test

import (
	"fmt"
	"testing"
)

func testTFVarInputSrcTF(t *testing.T, name, cmName string) string {
	tfapply := testMakeTF(t, tfSpecData{
		Kind:             TFKindApply,
		Name:             name,
		ConfigMapSources: []string{cmName},
		TFVars: map[string]string{
			"metadata_key": name,
		},
	})
	t.Log(tfapply)
	return tfapply
}

func testTFVarInputDestTF(t *testing.T, name, cmName string, tfinputs []TFInput) string {
	tfapply := testMakeTF(t, tfSpecData{
		Kind:             TFKindApply,
		Name:             name,
		ConfigMapSources: []string{cmName},
		TFInputs:         tfinputs,
	})
	t.Log(tfapply)
	return tfapply
}

func testTFVarInputVerifySrcTFApply(t *testing.T, name string) {
	tf := testWaitTF(t, TFKindApply, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionPodComplete,
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionReady,
	})
}

func testTFVarInputVerifyDestTFApply(t *testing.T, name, metadataKey1Value, metadataKey2Value string) {
	tf := testWaitTF(t, TFKindApply, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionPodComplete,
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionTFInputsReady,
		ConditionReady,
	})

	// Verify the output var value
	for _, output := range tf.Status.Outputs {
		if output.Name == "metadata_key" && metadataKey1Value != "" {
			assert(t, output.Value == metadataKey1Value, "tfapply source var value not found in output")
		}
		if output.Name == "metadata_key2" && metadataKey2Value != "" {
			assert(t, output.Value == metadataKey2Value, "tfapply source var value not found in output")
		}
	}
}

// TestTFVarInputMap runs a tfapply and uses the output variables as inputs to another TFApply.
func TestTFVarInputMap(t *testing.T) {
	t.Parallel()

	name := "tf-test-inputvar"
	srcName := fmt.Sprintf("%s-src", name)
	destName := fmt.Sprintf("%s-dest", name)

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	// Create the source tfapply.
	tfapplySrc := testTFVarInputSrcTF(t, srcName, name)
	defer testDelete(t, namespace, tfapplySrc)

	// Create the dest tf apply
	tfapplyDest := testTFVarInputDestTF(t, destName, name, []TFInput{
		TFInput{
			Name: srcName,
			VarMap: []InputVar{
				InputVar{
					Source: "metadata_key",
					Dest:   "metadata_key",
				},
			},
		},
	})
	defer testDelete(t, namespace, tfapplyDest)

	// Create both tfapply objects in parallel to verify the dest tfapply waits.

	// Apply the source tfapply
	testApply(t, namespace, tfapplySrc)

	// Apply the dest tfapply
	testApply(t, namespace, tfapplyDest)

	// Verify the source tfapply
	testTFVarInputVerifySrcTFApply(t, srcName)

	// Verify the dest tfapply
	testTFVarInputVerifyDestTFApply(t, destName, srcName, "")
}

// TestTFMultiVarInputMap verifies that a tfapply can accept vars from multiple other tfapply resources.
func TestTFMultiVarInputMap(t *testing.T) {
	t.Parallel()

	name := "tf-test-multi-inputvar"
	srcName1 := fmt.Sprintf("%s-src1", name)
	srcName2 := fmt.Sprintf("%s-src2", name)
	destName := fmt.Sprintf("%s-dest", name)

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	// Create the source1 tfapply.
	tfapplySrc1 := testTFVarInputSrcTF(t, srcName1, name)
	defer testDelete(t, namespace, tfapplySrc1)

	// Create the source2 tfapply.
	tfapplySrc2 := testTFVarInputSrcTF(t, srcName2, name)
	defer testDelete(t, namespace, tfapplySrc2)

	// Create the dest tf apply
	tfapplyDest := testTFVarInputDestTF(t, destName, name, []TFInput{
		TFInput{
			Name: srcName1,
			VarMap: []InputVar{
				InputVar{
					Source: "metadata_key",
					Dest:   "metadata_key",
				},
			},
		},
		TFInput{
			Name: srcName2,
			VarMap: []InputVar{
				InputVar{
					Source: "metadata_key",
					Dest:   "metadata_key2",
				},
			},
		},
	})
	defer testDelete(t, namespace, tfapplyDest)

	// Create all objects in parallel to verify the dest tfapply waits.

	// Apply the source tfapplys
	testApply(t, namespace, tfapplySrc1)
	testApply(t, namespace, tfapplySrc2)

	// Apply the test tfapply
	testApply(t, namespace, tfapplyDest)

	// Verify the source1 tfapply
	testTFVarInputVerifySrcTFApply(t, srcName1)
	testTFVarInputVerifySrcTFApply(t, srcName2)

	// Verify the dest tfapply
	testTFVarInputVerifyDestTFApply(t, destName, srcName1, srcName2)
}
