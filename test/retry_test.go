package test

import (
	"testing"
)

func testRetryTF(t *testing.T, kind TFKind, name, region string) string {
	embeddedSpec := helperLoadBytes(t, defaultTFSourcePath)
	tf := testMakeTF(t, tfSpecData{
		Kind:            kind,
		Name:            name,
		EmbeddedSources: []string{string(embeddedSpec)},
		TFVarsMap: map[string]string{
			"metadata_key": name,
			"region":       region,
		},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, kind, namespace, name)
	return tf
}

func TestRetry(t *testing.T) {
	name := "tf-test-retry"

	tfapply := testRetryTF(t, TFKindApply, name, "mars")
	defer testDelete(t, namespace, tfapply)

	tf := testGetTF(t, TFKindApply, namespace, name)
	tf.VerifyOutputVars(t)

	tfdestroy := testRetryTF(t, TFKindDestroy, name, "us-west1")
	defer testDelete(t, namespace, tfdestroy)
}
