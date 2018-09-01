variable "region" {
  default = "us-central1"
}

provider "google" {
  region = "${var.region}"
}

variable "network_name" {
  default = "example-tf-plugin"
}

resource "google_compute_network" "default" {
  name                    = "${var.network_name}"
  auto_create_subnetworks = "false"
}

resource "google_compute_subnetwork" "default" {
  name                     = "${var.network_name}"
  ip_cidr_range            = "10.127.0.0/20"
  network                  = "${google_compute_network.default.self_link}"
  region                   = "${var.region}"
  private_ip_google_access = true
}

output "network_region" {
  value = "${var.region}"
}

output "network_name" {
    value = "${google_compute_network.default.name}"
}