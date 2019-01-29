// Copyright 2017 CNI authors
// Modifications copyright (c) Linkerd authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file was inspired by:
// 1) https://github.com/istio/cni/blob/c63a509539b5ed165a6617548c31b686f13c2133/deployments/kubernetes/install/manifests/istio-cni.yaml

package install

// CNITemplate provides the base template for the `linkerd install-cni-plugin` command.
const CNITemplate = `### Namespace ###
kind: Namespace
apiVersion: v1
metadata:
  name: {{.Namespace}}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: linkerd-cni
  namespace: {{.Namespace}}
---
# Include a clusterrole for the linkerd CNI DaemonSet,
# and bind it to the linkerd-cni serviceaccount.
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: linkerd-cni
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "namespaces"]
    verbs: ["list", "get", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: linkerd-cni
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: linkerd-cni
subjects:
- kind: ServiceAccount
  name: linkerd-cni
  namespace: {{.Namespace}}
---
# This ConfigMap is used to configure a self-hosted linkerd CNI installation.
kind: ConfigMap
apiVersion: v1
metadata:
  name: linkerd-cni-config
  namespace: {{.Namespace}}
data:
  incoming_proxy_port: "{{.InboundPort}}"
  outgoing_proxy_port: "{{.OutboundPort}}"
  proxy_uid: "{{.ProxyUID}}"
  inbound_ports_to_ignore: "{{.IgnoreInboundPorts}}"
  outbound_ports_to_ignore: "{{.IgnoreOutboundPorts}}"
  simulate: "false"
  log_level: "{{.LogLevel}}"
  # The CNI network configuration to install on each node. The special
  # values in this config will be automatically populated.
  cni_network_config: |-
    {
      "name": "linkerd-cni",
      "type": "linkerd-cni",
      "log_level": "__LOG_LEVEL__",
      "policy": {
          "type": "k8s",
          "k8s_api_root": "https://__KUBERNETES_SERVICE_HOST__:__KUBERNETES_SERVICE_PORT__",
          "k8s_auth_token": "__SERVICEACCOUNT_TOKEN__"
      },
      "kubernetes": {
          "kubeconfig": "__KUBECONFIG_FILEPATH__"
      },
      "linkerd": {
        "incoming-proxy-port": __INCOMING_PROXY_PORT__,
        "outgoing-proxy-port": __OUTGOING_PROXY_PORT__,
        "proxy-uid": __PROXY_UID__,
        "ports-to-redirect": [__PORTS_TO_REDIRECT__],
        "inbound-ports-to-ignore": [__INBOUND_PORTS_TO_IGNORE__],
        "outbound-ports-to-ignore": [__OUTBOUND_PORTS_TO_IGNORE__],
        "simulate": __SIMULATE__
      }
    }
---
# This manifest installs the linkerd CNI plugins and network config on
# each master and worker node in a Kubernetes cluster.
kind: DaemonSet
apiVersion: extensions/v1beta1
metadata:
  name: linkerd-cni
  namespace: {{.Namespace}}
  labels:
    k8s-app: linkerd-cni
spec:
  selector:
    matchLabels:
      k8s-app: linkerd-cni
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  template:
    metadata:
      labels:
        k8s-app: linkerd-cni
      annotations:
        # This, along with the CriticalAddonsOnly toleration below,
        # marks the pod as a critical add-on, ensuring it gets
        # priority scheduling and that its resources are reserved
        # if it ever gets evicted.
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      nodeSelector:
        beta.kubernetes.io/os: linux
      hostNetwork: true
      tolerations:
        # Make sure linkerd-cni gets scheduled on all nodes.
        - effect: NoSchedule
          operator: Exists
        # Mark the pod as a critical add-on for rescheduling.
        - key: CriticalAddonsOnly
          operator: Exists
        - effect: NoExecute
          operator: Exists
      serviceAccountName: linkerd-cni
      terminationGracePeriodSeconds: 5
      containers:
        # This container installs the linkerd CNI binaries
        # and CNI network config file on each node. The install
        # script copies the files into place and then sleeps so
        # that Kubernetes doesn't keep trying to restart it.
        - name: install-cni
          image: {{.CNIPluginImage}}
          env:
            # The CNI network config to install on each node.
            - name: CNI_NETWORK_CONFIG
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: cni_network_config
            - name: INCOMING_PROXY_PORT
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: incoming_proxy_port
            - name: OUTGOING_PROXY_PORT
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: outgoing_proxy_port
            - name: PROXY_UID
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: proxy_uid
            - name: INBOUND_PORTS_TO_IGNORE
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: inbound_ports_to_ignore
            - name: LOG_LEVEL
              valueFrom:
                configMapKeyRef:
                  name: linkerd-cni-config
                  key: log_level
            - name: SLEEP
              value: "true"
          volumeMounts:
            - mountPath: /host/opt/cni/bin
              name: cni-bin-dir
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
      volumes:
        # Used to install CNI.
        - name: cni-bin-dir
          hostPath:
            path: /opt/cni/bin
        - name: cni-net-dir
          hostPath:
            path: /etc/cni/net.d
`
