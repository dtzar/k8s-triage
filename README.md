# Kubernetes boards triage

Tool to automate triage of boards in K8s SIG Node.

The idea is to implement project board management rules like described
in this [proposal](https://github.com/kubernetes/community/issues/6999).

Run locally:

```
ACCESS_TOKEN=ghp_FooBar PORT=8080 go run main.go
```

This app is deployed at https://apmtips.com/triage/node-prs
