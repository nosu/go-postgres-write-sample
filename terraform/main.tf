variable "project_id" {
  default = "alloydb-test-381909"
}

variable "region" {
  default = "us-central1"
}

variable "zone" {
  default = "us-central1-c"
}

variable "alloydb_user" {
  default = "alloydb-user"
}

variable "alloydb_password" {}

data "google_project" "project" {}

provider "google" {
  project = "${var.project_id}"
  region  = "${var.region}"
  zone    = "${var.zone}"
}

variable "gcp_service_list" {
  description ="The list of apis necessary for the project"
  type = list(string)
  default = [
    "vpcaccess.googleapis.com",
    "artifactregistry.googleapis.com"
  ]
}

resource "google_project_service" "gcp_services" {
  for_each = toset(var.gcp_service_list)
  project = var.project_id
  service = each.key
}

module "vpc" {
  source  = "terraform-google-modules/network/google"
  version = "~> 4.0"

  project_id   = var.project_id
  network_name = "alloydb-vpc"
  routing_mode = "GLOBAL"

  subnets = [
    {
      subnet_name           = "cloud-run-subnet"
      subnet_ip             = "10.10.0.0/28"
      subnet_region         = "us-central1"
      subnet_private_access = "true"
      subnet_flow_logs      = "false"
      description           = "Cloud Run VPC Connector Subnet"
    }
  ]
}

module "serverless_connector" {
  source  = "terraform-google-modules/network/google//modules/vpc-serverless-connector-beta"
  version = "~> 4.0"

  project_id = var.project_id
  vpc_connectors = [{
    name            = "central-serverless"
    region          = "us-central1"
    subnet_name     = "${module.vpc.subnets["us-central1/cloud-run-subnet"]["name"]}"
    host_project_id = var.project_id
    machine_type    = "e2-micro"
    min_instances   = 2
    max_instances   = 3
  }]
}

module "service_account" {
  source     = "terraform-google-modules/service-accounts/google"
  version    = "~> 4.1.1"
  project_id = var.project_id
  prefix     = "sa-cloud-run"
  names      = ["vpc-connector"]
}

module "cloud_run" {
  source  = "GoogleCloudPlatform/cloud-run/google"
  version = "~> 0.4.0"

  # Required variables
  service_name           = "db-write-webapi"
  project_id             = var.project_id
  location               = var.region
  image                  = "gcr.io/cloudrun/hello"
  service_account_email = module.service_account.email

  members = ["allUsers"]
  limits = {
    cpu = 1
    memory = "256Mi"
  }

  env_vars = [
    { 
      name  = "PROJECT_ID"
      value = var.project_id
    }, { 
      name  = "DB_USER"
      value = var.alloydb_user
    }, { 
      name  = "DB_PASS"
      value = var.alloydb_password
    }, { 
      name  = "DB_HOST"
      value = google_compute_global_address.private_ip_alloc.address
    }, { 
      name  = "DB_PORT"
      value = "5432"
    }, { 
      name  = "DB_NAME"
      value = "test_db"
    }
  ]

  template_annotations = {
    "autoscaling.knative.dev/maxScale"        = 1
    "autoscaling.knative.dev/minScale"        = 0
    "run.googleapis.com/vpc-access-connector" = element(tolist(module.serverless_connector.connector_ids), 1)
    "run.googleapis.com/vpc-access-egress"    = "all-traffic"
  }
}

module "alloy-db" {
  source               = "github.com/GoogleCloudPlatform/terraform-google-alloy-db"
  project_id           = var.project_id
  cluster_id           = "alloydb-cluster-with-prim"
  cluster_location     = "us-central1"
  cluster_labels       = {}
  cluster_display_name = ""
  cluster_initial_user = {
    user     = var.alloydb_user,
    password = var.alloydb_password
  }
  # network_self_link = "projects/${var.project_id}/global/networks/${var.network_name}"
  network_self_link = "projects/${var.project_id}/global/networks/alloydb-vpc"

  automated_backup_policy = null

  primary_instance = {
    instance_id       = "primary-instance",
    instance_type     = "PRIMARY",
    machine_cpu_count = 2,
    database_flags    = {},
    display_name      = "alloydb-primary-instance"
  }

  read_pool_instance = null

  # depends_on = [google_compute_network.default, google_compute_global_address.private_ip_alloc, google_service_networking_connection.vpc_connection]
  depends_on = [module.vpc, module.serverless_connector, google_compute_global_address.private_ip_alloc, google_service_networking_connection.vpc_connection]
}

resource "google_compute_global_address" "private_ip_alloc" {
  project       = var.project_id
  name          = "adb-v6"
  address_type  = "INTERNAL"
  purpose       = "VPC_PEERING"
  prefix_length = 16
  # network       = google_compute_network.default.id
  network       = module.vpc.network_name
}

resource "google_service_networking_connection" "vpc_connection" {
  network                 = module.vpc.network_name
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_alloc.name]
}

# resource "google_artifact_registry_repository" "repo" {
#   location      = var.region
#   repository_id = "docker"
#   description   = "docker repository"
#   format        = "DOCKER"
# }