---
type: Git
date: 2023
Context: k8soperator
Provider: localhost
---

### Minikube

Start a local Kubernetes cluster.

```bash
# Use virtual machines like with the VirtualBox or Hyper-V drivers
minikube start  --nodes=3 --memory=2g --cpus=2

# Use Docker as the driver for creating the Kubernetes node VMs (like MAC M1)
minikube start  --nodes=3 --driver=docker --memory=2g --cpus=2
```

Check status of cluster.

```bash
minikube status

kubectl get nodes -o wide
```

### Pod Restart Notifier Operator

The purpose of this operator is check the state of all pods every minute and send a notification  via a specified channel (e.g., Discord) for every detected restart of a pod. 

Initialise a new Go module

```bash
go mod init pod-restart-notifier
```

Install Dependencies

```bash
go get github.com/bwmarrin/discordgo@v0.27.1

go get \
k8s.io/apimachinery/pkg/apis/meta/v1@v0.28.0 \
k8s.io/apimachinery/pkg/api/resource@v0.28.0 \
k8s.io/apimachinery/pkg/util/net@v0.28.0 \
k8s.io/client-go/rest@v0.28.0 \
k8s.io/apimachinery/pkg/labels@v0.28.0 \
k8s.io/apimachinery/pkg/util/json@v0.28.0 \
k8s.io/apimachinery/pkg/runtime@v0.28.0 \
k8s.io/client-go/discovery@v0.28.0 \
k8s.io/apimachinery/pkg/runtime/serializer/json@v0.28.0 \
k8s.io/apimachinery/pkg/util/managedfields@v0.28.0 \
k8s.io/client-go/openapi@v0.28.0 \
k8s.io/client-go/plugin/pkg/client/auth/exec@v0.28.0 \
k8s.io/apimachinery/pkg/util/dump@v0.28.0 \
k8s.io/client-go/transport@v0.28.0 \
k8s.io/client-go/util/flowcontrol@v0.28.0
```

Build the Operator

```bash
go build -o pod-restart-notifier .
```
### Build the Docker image and push to a container registry

Create docker file

```Dockerfile
# Use the official Go image as a parent image
FROM golang:1.21 as builder

# Set the working directory in the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN go build -o pod-restart-notifier .

# Use a smaller base image for the final layer
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the builder layer
COPY --from=builder /app/pod-restart-notifier .

# Command to run the binary
CMD ["./pod-restart-notifier"]
```

Build docker image

```bash
docker build -t <your_dockerhub_username>/pod-restart-notifier-operator:0.1.4 .
```

Push the Docker image to a container registry, for example Docker Hub:

```bash
# If you're not already logged in:
docker login

# Push the versioned image
docker push <your_dockerhub_username>/pod-restart-notifier-operator:0.1.4
```

Replace `<your_dockerhub_username>` with your Docker Hub username.

### Setting up Helm Chart Structure

Create a new directory structure for the Helm chart:

```bash
mkdir pod-restart-notifier
cd pod-restart-notifier
mkdir templates
touch Chart.yaml
touch values.yaml
```

1. Edit the `Chart.yaml`:

`Chart.yaml` describes the metadata about the Helm chart:

```yaml
apiVersion: v2
name: pod-restart-notifier
description: A Helm chart for a Kubernetes operator that notifies on pod restarts via Discord, Microsoft Teams and Slack.
type: application
version: 0.1.5
appVersion: 1.0.1
maintainers:
- name: Nikos Nikolakakis
home: https://github.com/NoNickeD/pod-restart-notifier-operator
sources:
- https://github.com/NoNickeD/pod-restart-notifier-operator
dependencies: []
```

2. Set Up Default `values.yaml`:

This file will contain default values which users of the chart can override:

```yaml
image:
  repository: nonickednn/pod-restart-notifier-operator
  tag: 0.1.5

discord:
  webhookURL: "DISCORD_WEBHOOK_URL"

teams:
  webhookURL: "TEAMS_WEBHOOK_URL"

slack:
  webhookURL: "SLACK_WEBHOOK_URL"
```

Make sure to replace placeholders (`<your_dockerhub_username>` and `YOUR_DISCORD_WEBHOOK_URL`) with appropriate values.

3. Templates for the RBAC and Deployment:

Under the `templates` directory, create the Kubernetes resource files:

**Service Account** (`templates/serviceaccount.yaml`):

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-restart-notifier-operator-sa
  namespace: "{{ .Values.namespace }}"
```

**Cluster Role** (`templates/clusterrole.yaml`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pod-restart-notifier-operator-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
```

**Cluster Role Binding** (`templates/clusterrolebinding.yaml`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pod-restart-notifier-operator-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: pod-restart-notifier-operator-role
subjects:
- kind: ServiceAccount
  name: pod-restart-notifier-operator-sa
  namespace: "{{ .Values.namespace }}"
```

**Deployment** (`templates/deployment.yaml`):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "{{ .Release.Name }}"
  namespace: "{{ .Values.namespace }}"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pod-restart-notifier-operator
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%
  template:
    metadata:
      labels:
        app: pod-restart-notifier-operator
    spec:
      serviceAccountName: pod-restart-notifier-operator-sa
      containers:
      - name: operator
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        env:
        - name: DISCORD_WEBHOOK_URL
          value: "{{ .Values.discord.webhookURL }}"
        - name: TEAMS_WEBHOOK_URL
          value: "{{ .Values.teams.webhookURL }}"
        - name: SLACK_WEBHOOK_URL
          value: "{{ .Values.slack.webhookURL }}"
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

**NOTES** (`templates/NOTES.txt`):

```text
Thank you for installing the Pod Restart Notifier!

1. To check the deployment:
    $ kubectl get deployments -n {{ .Release.Namespace }} {{ .Release.Name }}

2. To see the operator logs:
    $ kubectl logs -n {{ .Release.Namespace }} -l app=pod-restart-notifier

3. To verify the operator's functionality, you can induce a restart in a pod and then check the operator logs or your Discord channel, Microsoft Teams, or Slack for notifications.

Setup:
- Ensure you have your Discord webhook URL set up to receive the notifications.
- For Microsoft Teams, set up an incoming webhook connector in your Teams channel to get a webhook URL.
- For Slack, create an incoming webhook from the "Apps" section in your Slack workspace to receive a webhook URL.

Configuration:
- If you need to adjust the webhook URLs, you can update the Helm release or modify the `values.yaml`.

Happy monitoring!
```

The deployment will utilize the webhook URL from the `values.yaml` as an environment variable. Adjust your operator code to read this value from the environment.

Create the `test-notifier` namespace:

```bash
kubectl create namespace test-notifier
```

Deploy the Testing Pods:

- We will deploy 2 pods that will automatically exit roughly every 3 minutes and 20 seconds, prompting Kubernetes to restart them.

Create the `restart-deployment.yaml`:

```bash
touch restart-deployment.yaml

vi restart-deployment.yaml
```

Paste the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restart-deployment
  namespace: test-notifier
spec:
  replicas: 2
  selector:
    matchLabels:
      app: restart-pod
  template:
    metadata:
      labels:
        app: restart-pod
    spec:
      containers:
      - name: restart-container
        image: busybox
        command: ["/bin/sh"]
        args: ["-c", "sleep 200; exit 0"]
```

Apply the deployment:

```bash
kubectl apply -f restart-deployment.yaml
```

Monitor the Restarts:

You can observe the pods and their restart counts by running:

```bash
kubectl get pods -n test-notifier -w
```

- We will deploy 1 pods that will automatically exit prompting Kubernetes to restart them.

Create the `failing-pod-deploy.yaml`:

```bash
touch failing-pod-deploy.yaml

vi failing-pod-deploy.yaml
```

Paste the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: failing-pod
  namespace: test-notifier
spec:
  replicas: 1
  selector:
    matchLabels:
      app: failing-app
  template:
    metadata:
      labels:
        app: failing-app
    spec:
      containers:
      - name: failing-container
        image: busybox
        command:
        - /bin/sh
        - -c
        - "exit 1"
```

Apply the deployment:

```bash
kubectl apply -f failing-pod-deploy.yaml
```

Monitor the Restarts:

You can observe the pods and their restart counts by running:

```bash
kubectl get pods -n test-notifier -w
```

### Install the Helm Chart:

First, lint the chart to ensure no errors:

```bash
helm lint ./pod-restart-notifier
```

Then, package and install:

```bash
# Package
helm package ./pod-restart-notifier

# Basic installation using the default values specified in the chart's values.yaml file.
helm install pod-restart-notifier ./pod-restart-notifier-0.1.5.tgz
```

To override values during installation:

```bash
# Discord
helm install pod-restart-notifier ./pod-restart-notifier-0.1.5.tgz --namespace "YOUR_NAMESPACE" --set discord.webhookURL="WEBHOOK_URL"

helm install pod-restart-notifier ./pod-restart-notifier-0.1.5.tgz --namespace=test-notifier --set discord.webhookURL="
https://discord.com/api/webhooks/1143146589075030047/OVg9glpBo2IwZbCboxM873s1mxOEflFdT34hfzbTdVx75RwMRuOVronrKW9FuepATwwP"

# Teams
helm install pod-restart-notifier ./pod-restart-notifier-0.1.5.tgz --namespace "YOUR_NAMESPACE" --set teams.webhookURL="WEBHOOK_URL"
```

The structure of  Helm chart must be the following:

```bash
./pod-restart-notifier
├── Chart.yaml
├── templates
│   ├── NOTES.txt
│   ├── clusterrole.yaml
│   ├── clusterrolebinding.yaml
│   ├── deployment.yaml
│   └── serviceaccount.yaml
└── values.yaml
```
