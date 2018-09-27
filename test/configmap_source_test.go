package test

import (
	"testing"
)

func testConfigMapSourceTF(t *testing.T, kind TFKind, name, cmName string, delete bool) string {
	spec := testMakeTF(t, tfSpecData{
		Kind:             kind,
		Name:             name,
		ConfigMapSources: []string{cmName},
		TFVars: map[string]string{
			"metadata_key": name,
		},
	})
	if delete {
		defer testDelete(t, namespace, spec)
	}

	t.Log(spec)
	testApply(t, namespace, spec)
	tf := testWaitTF(t, kind, namespace, name)
	tf.VerifyConditions(t, []ConditionType{
		ConditionPodComplete,
		ConditionProviderConfigReady,
		ConditionSourceReady,
		ConditionReady,
	})
	return spec
}

// TestConfigMapSource runs a plan,apply,destroy in sequence using a configmap source.
func TestConfigMapSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-cm"

	testApplyTFSourceConfigMap(t, namespace, name)
	defer testDeleteTFSourceConfigMap(t, namespace, name)

	testConfigMapSourceTF(t, TFKindPlan, name, name, true)
	testConfigMapSourceTF(t, TFKindApply, name, name, true)
	testConfigMapSourceTF(t, TFKindDestroy, name, name, true)
}
