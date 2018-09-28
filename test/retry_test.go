package test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func testMakeRetryTF(t *testing.T, kind TFKind, name, region string) string {
	embeddedSpec := helperLoadBytes(t, defaultTFSourcePath)
	tf := testMakeTF(t, tfSpecData{
		Kind:            kind,
		Name:            name,
		EmbeddedSources: []string{string(embeddedSpec)},
		TFVars: map[string]string{
			"metadata_key": name,
			"region":       region,
		},
	})
	t.Log(tf)
	return tf
}

func TestRetry(t *testing.T) {
	t.Parallel()

	name := "tf-test-retry"

	tfapply := testMakeRetryTF(t, TFKindApply, name, "mars")
	defer testDelete(t, namespace, tfapply)

	testApply(t, namespace, tfapply)

	// Wait for max number of retries plus 1
	maxRetries := 5
	baseNamePat := regexp.MustCompile(fmt.Sprintf(`^(%s)-tfapply-([0-9])$`, name))
	for {
		tf := testGetTF(t, TFKindApply, namespace, name)
		if tf.Status.PodName == "" {
			fmt.Printf("Waiting for %s/%s pod: %s %s\n", TFKindApply, name, tf.Status.PodName, tf.Status.PodStatus)
			time.Sleep(time.Second * time.Duration(5))
		} else {
			assert(t, baseNamePat.MatchString(tf.Status.PodName), "invalid pod name: %s", tf.Status.PodName)
			baseNamePat.FindAllStringSubmatch(tf.Status.PodName, 0)
			match := baseNamePat.FindStringSubmatch(tf.Status.PodName)
			index, _ := strconv.Atoi(match[2])
			if index >= maxRetries {
				fmt.Printf("Found %d pod attempts\n", index)
				break
			} else {
				fmt.Printf("Waiting on %d pod attempts, seen %d/%d\n", maxRetries, index, maxRetries)
				time.Sleep(time.Second * time.Duration(5))
			}
		}
	}

	// Patch the tfapply object to make it pass.
	tfapply = testMakeRetryTF(t, TFKindApply, name, "us-west1")
	testApply(t, namespace, tfapply)
	testWaitTF(t, TFKindApply, namespace, name)
	testVerifyOutputVars(t, namespace, name)

	tfdestroy := testMakeRetryTF(t, TFKindDestroy, name, "us-west1")
	testApply(t, namespace, tfdestroy)
	testWaitTF(t, TFKindDestroy, namespace, name)
	defer testDelete(t, namespace, tfdestroy)
}
