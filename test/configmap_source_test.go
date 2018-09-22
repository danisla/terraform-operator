package test

import (
	"testing"
)

func testConfigMapSourceTFPlan(t *testing.T, name, cmName string) {
	tf := testMakeTF(t, tfSpecData{
		Kind:             "TerraformPlan",
		Name:             name,
		ConfigMapSources: []string{cmName},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, "tfplan", namespace, name)
	defer testDelete(t, namespace, tf)
}

func testConfigMapSourceTFApply(t *testing.T, name, cmName string) {
	tf := testMakeTF(t, tfSpecData{
		Kind:             "TerraformApply",
		Name:             name,
		ConfigMapSources: []string{cmName},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, "tfapply", namespace, name)
	defer testDelete(t, namespace, tf)
}

func testConfigMapSourceTFDestroy(t *testing.T, name, cmName string) {
	tf := testMakeTF(t, tfSpecData{
		Kind:             "TerraformDestroy",
		Name:             name,
		ConfigMapSources: []string{cmName},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, "tfdestroy", namespace, name)
	defer testDelete(t, namespace, tf)
}

// TestConfigMapSource runs a plan,apply,destroy in sequence using a configmap source.
func TestConfigMapSource(t *testing.T) {
	name := "tf-test-cm"
	cmName := "tf-test-cm"
	testApplyTFSourceConfigMap(t, namespace, cmName)
	defer testDeleteTFSourceConfigMap(t, namespace, cmName)

	testConfigMapSourceTFPlan(t, name, cmName)
	testConfigMapSourceTFApply(t, name, cmName)
	testConfigMapSourceTFDestroy(t, name, cmName)
}
