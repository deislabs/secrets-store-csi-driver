#!/usr/bin/env bats

load helpers

@test "csi-secrets-store-attacher-0 is running" {
  cmd="kubectl wait -n dev --for=condition=Ready --timeout=60s pod/csi-secrets-store-attacher-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/csi-secrets-store-attacher-0
  assert_success
}

@test "install helm chart with local image" {
  run helm install charts/secrets-store-csi-driver -n csi-secrets-store --namespace dev \
          --set provider="" \
          --set image.pullPolicy="IfNotPresent" \
          --set image.repository=e2e/secrets-store-csi \
          --set image.tag=$(git rev-parse --short HEAD)
  assert_success
}

@test "create azure secret" {
  run kubectl create secret generic secrets-store-creds --from-literal clientid=$AZURE_CLIENT_ID --from-literal clientsecret=$AZURE_CLIENT_SECRET
  assert_success
}

@test "CSI inline volume test" {
  run kubectl apply -f test/bats/tests/nginx-pod-secrets-store-inline-volume.yaml
  assert_success

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline
  assert_success
}

@test "read azure kv secret from pod" {
  result=$(kubectl exec -it nginx-secrets-store-inline cat /mnt/secrets-store/secret1)
  [[ "$result" -eq "test" ]]
}

@test "read azure kv key from pod" {
  result=$(kubectl exec -it nginx-secrets-store-inline cat /mnt/secrets-store/key1)
  [[ "$result" -eq "test" ]]
}