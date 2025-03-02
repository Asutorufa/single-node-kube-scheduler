
provider "kubernetes" {
  config_path = "~/.kube/config"
}

resource "kubernetes_service_account" "single-node-kube-scheduler" {
  metadata {
    name      = var.name
    namespace = var.namespace
  }
}

resource "kubernetes_cluster_role" "single-node-kube-scheduler" {
  metadata {
    name = var.name
  }

  rule {
    api_groups = [""]
    resources  = ["pods", "pods/binding"]
    // create,delete,deletecollection,get,list,patch,update,watch
    verbs = ["get", "list", "watch", "delete", "deletecollection", "update", "patch", "create"]
  }

  rule {
    api_groups = [""]
    resources  = ["nodes"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "single-node-kube-scheduler" {
  metadata {
    name = var.name
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.single-node-kube-scheduler.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.single-node-kube-scheduler.metadata[0].name
    namespace = var.namespace
  }
}

resource "kubernetes_daemonset" "single-node-kube-scheduler" {
  metadata {
    name      = var.name
    namespace = var.namespace
  }

  spec {
    selector {
      match_labels = {
        app = var.name
      }
    }

    template {
      metadata {
        labels = {
          app = var.name
        }
      }

      spec {
        node_name            = var.node_name
        service_account_name = kubernetes_service_account.single-node-kube-scheduler.metadata[0].name
        container {
          name              = var.name
          image             = "ghcr.io/asutorufa/single-node-kube-scheduler:latest"
          image_pull_policy = "IfNotPresent"
        }
      }
    }
  }
}
