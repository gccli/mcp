# Gitlab Runner

## Install GitLab Runner with the Helm chart


https://docs.gitlab.com/runner/install/kubernetes/

```
kubectl create ns gitlab

helm repo add gitlab https://charts.gitlab.io

helm install --namespace gitlab gitlab-runner -f values.yaml gitlab/gitlab-runner

```


## Upgrade GitLab Runner with the Helm chart

```
helm upgrade --namespace gitlab -f values.yaml gitlab-runner gitlab/gitlab-runner
```



# FAQ

如何让 GitLab Kubernetes Runner（简称 K8S Runner）将流水线任务（Job）调度到 Kubernetes 集群的指定节点上？

方法一：修改全局配置文件 config.toml
https://docs.gitlab.com/runner/executors/kubernetes/#specify-the-node-to-execute-builds

 [runners.kubernetes.node_selector]
   "kubernetes.io/arch" = "arm64"
   "kubernetes.io/os" = "linux"


方法二：在 `.gitlab-ci.yml` 中动态覆盖（推荐：更灵活）如果您使用的是通过 Helm 部署的比较新的 GitLab Runner


```
stages:
- build

compile_job:
  stage: build
  image: maven:3.8-openjdk-11
  variables:

  # 假设节点标签是 jobs=ci
  KUBERNETES_NODE_SELECTOR_jobs: "ci"
  script:
  - mvn clean package
```
