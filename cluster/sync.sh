#!/bin/bash -xe
sidecar_image=quay.io/ellorent/kubevirt-passt-binding
sidecar_image_sha=$(skopeo inspect docker://$sidecar_image | jq -r .Digest)

kubectl patch kubevirts -n kubevirt kubevirt --type=json -p="[{\"op\": \"add\", \"path\": \"/spec/configuration/network\",   \"value\": {
      \"binding\": {
          \"passt\": {
              \"sidecarImage\": \"${sidecar_image}@${sidecar_image_sha}\",
              \"downwardAPI\": \"device-info\",
              \"migration\": {
                  \"method\": \"link-refresh\"
              }
          }
      }
  }}]"

kubectl apply -f - <<EOF
---
apiVersion: kubevirt.io/v1
kind: KubeVirt
metadata:
  name: kubevirt
  namespace: kubevirt
spec:
  configuration:
    developerConfiguration:
      featureGates:
        - NetworkBindingPlugins
EOF

for node in $(kubectl get node --no-headers  -o custom-columns=":metadata.name"); do
    docker cp ./kubevirt-passt-binding $node:/opt/cni/bin/kubevirt-passt-binding
done
