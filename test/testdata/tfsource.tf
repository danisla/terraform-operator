variable "region" {
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
}