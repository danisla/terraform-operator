variable "region" {
  default = "us-central1"
}
variable "escape_test" {
  default = "%"
}
variable "metadata_key" {
  default = "tf-test-key"
}
variable "metadata_key2" {
  default = "tf-test-key2"
}
provider "google" {
  region = "${var.region}"
}
data "google_compute_zones" "available" {
  region = "${var.region}"
}
resource "google_compute_project_metadata_item" "default" {
  key = "${var.metadata_key}"
  value = "tf-operator-test-${element(data.google_compute_zones.available.names, 0)}"
}
resource "google_compute_project_metadata_item" "default2" {
  key = "${var.metadata_key2}"
  value = "tf-operator-test-${element(data.google_compute_zones.available.names, 0)}"
}
data "google_client_config" "current" {}
output "project" {
  value = "${data.google_client_config.current.project}"
}
output "region" {
  value = "${var.region}"
}
output "zones" {
  value = "${join(",", data.google_compute_zones.available.names)}"
}
output "metadata_key" {
  value = "${google_compute_project_metadata_item.default.key}"
}
output "metadata_key2" {
  value = "${google_compute_project_metadata_item.default2.key}"
}
output "metadata_value" {
  value = "${google_compute_project_metadata_item.default.value}"
}