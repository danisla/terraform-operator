package test

import (
	"testing"
)

func testRetryTF(t *testing.T, kind TFKind, name, region string) {
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
	defer testDelete(t, namespace, tf)
}

func TestRetry(t *testing.T) {
	name := "tf-test-retry"

	testRetryTF(t, TFKindApply, name, "mars")
	testRetryTF(t, TFKindDestroy, name, "us-west1")
}
