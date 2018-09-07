package test

import (
	"testing"
)

func TestConfigMapSourceTFPlan(t *testing.T) {
	testApplyTFSourceConfigMap(t, "default", "job1-cm-tf")
	defer testDeleteTFSourceConfigMap(t, "default", "job1-cm-tf")

	tf := testMakeTF(t, tfSpecData{
		Kind: "TerraformPlan",
		Name: "job1-cm",
	})

	t.Fatalf(string(tf))
}
