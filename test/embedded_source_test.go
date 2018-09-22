package test

import (
	"testing"
)

const embeddedSpec string = `variable "region" {
  default = "us-central1"
}
variable "escape_test" {
  default = "%"
}
provider "google" {
  region = "${var.region}"
}
resource "google_compute_project_metadata_item" "default" {
  key = "tf-job-test"
  value = "tf-operator-test"
}
data "google_client_config" "current" {}
output "project" {
  value = "${data.google_client_config.current.project}"
}
output "region" {
  value = "${var.region}"
}
output "metadata_key" {
  value = "${google_compute_project_metadata_item.default.key}"
}
output "metadata_value" {
  value = "${google_compute_project_metadata_item.default.value}"
}`

func testEmbeddedSourceTF(t *testing.T, kind TFKind, name string) {
	tf := testMakeTF(t, tfSpecData{
		Kind:            kind,
		Name:            name,
		EmbeddedSources: []string{embeddedSpec},
		TFVarsMap: map[string]string{
			"region": "us-west1",
		},
	})
	t.Log(tf)
	testApply(t, namespace, tf)
	testWaitTF(t, kind, namespace, name)
	defer testDelete(t, namespace, tf)
}

// TestEmbeddedSource runs a plan,apply,destroy in serial with an embedded terraform source.
func TestEmbeddedSource(t *testing.T) {
	t.Parallel()

	name := "tf-test-src"
	testEmbeddedSourceTF(t, TFKindPlan, name)
	testEmbeddedSourceTF(t, TFKindApply, name)
	testEmbeddedSourceTF(t, TFKindDestroy, name)
}

func TestEmbeddedSourceFromTFPlanAndTFApply(t *testing.T) {
	t.Parallel()

	name := "tf-test-src-tfplan-or-tfapply"
	testEmbeddedSourceTF(t, TFKindPlan, name)
	testEmbeddedSourceTF(t, TFKindApply, name)

	// Test with both tfapply and tfplan present.
	// Create tfdestroy
	tfdestroy := testMakeTF(t, tfSpecData{
		Kind: TFKindDestroy,
		Name: name,
		TFSources: []map[string]string{
			map[string]string{
				"tfplan":  name,
				"tfapply": name,
			},
		},
	})
	t.Log(tfdestroy)
	testApply(t, namespace, tfdestroy)
	testWaitTF(t, TFKindDestroy, namespace, name)
	defer testDelete(t, namespace, tfdestroy)

}
