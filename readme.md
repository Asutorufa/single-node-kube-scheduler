# single node kube scheduler

Disabled kube-scheduler and just bind one exist node to pods

## install

disable kube-scheduler first, then:

```shell
cd infra
terraform init
# here node name for run single-node-kube-scheduler self
terraform apply -var="node_name=node_name"
```
