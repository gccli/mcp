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
