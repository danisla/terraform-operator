variable "region" {}
variable "network_name" {}
variable "mig_size" {}

data "template_file" "group-startup-script" {
  template = "${file("${format("%s/gceme.sh.tpl", path.module)}")}"

  vars {
    PROXY_PATH = ""
  }
}

module "mig1" {
  source            = "GoogleCloudPlatform/managed-instance-group/google"
  version           = "1.1.13"
  zonal             = false
  region            = "${var.region}"
  name              = "${var.network_name}"
  size              = "${var.mig_size}"
  target_tags       = ["${var.network_name}"]
  service_port      = 80
  service_port_name = "http"
  http_health_check = false
  startup_script    = "${data.template_file.group-startup-script.rendered}"
  network           = "${var.network_name}"
  subnetwork        = "${var.network_name}"
}

resource "google_compute_firewall" "default" {
  name    = "${var.network_name}-http"
  network = "${var.network_name}"

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["80"]
  }

  source_tags = ["${var.network_name}"]
}