#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/azure
WAIT_TIME=60
SLEEP_TIME=1
NAMESPACE=kube-system
PROVIDER_YAML=https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer.yaml
NODE_SELECTOR_OS=linux
BASE64_FLAGS="-w 0"
if [[ "$OSTYPE" == *"darwin"* ]]; then
  BASE64_FLAGS="-b 0"
fi

if [ $TEST_WINDOWS ]; then
  PROVIDER_YAML=https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer-windows.yaml
  NODE_SELECTOR_OS=windows
fi

if [ -z "$AUTO_ROTATE_SECRET_NAME" ]; then
    export AUTO_ROTATE_SECRET_NAME=secret-$(openssl rand -hex 6)
fi

export KEYVAULT_NAME=${KEYVAULT_NAME:-csi-secrets-store-e2e}
export SECRET_NAME=${KEYVAULT_SECRET_NAME:-secret1}
export SECRET_VERSION=${KEYVAULT_SECRET_VERSION:-""}
export SECRET_VALUE=${KEYVAULT_SECRET_VALUE:-"test"}
export KEY_NAME=${KEYVAULT_KEY_NAME:-key1}
export KEY_VERSION=${KEYVAULT_KEY_VERSION:-7cc095105411491b84fe1b92ebbcf01a}
export KEY_VALUE_CONTAINS=${KEYVAULT_KEY_VALUE:-"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF4K2FadlhJN2FldG5DbzI3akVScgpheklaQ2QxUlBCQVZuQU1XcDhqY05TQk5MOXVuOVJrenJHOFd1SFBXUXNqQTA2RXRIOFNSNWtTNlQvaGQwMFNRCk1aODBMTlNxYkkwTzBMcWMzMHNLUjhTQ0R1cEt5dkpkb01LSVlNWHQzUlk5R2Ywam1ucHNKOE9WbDFvZlRjOTIKd1RINXYyT2I1QjZaMFd3d25MWlNiRkFnSE1uTHJtdEtwZTVNcnRGU21nZS9SL0J5ZXNscGU0M1FubnpndzhRTwpzU3ZMNnhDU21XVW9WQURLL1MxREU0NzZBREM2a2hGTjF5ZHUzbjVBcnREVGI0c0FjUHdTeXB3WGdNM3Y5WHpnClFKSkRGT0JJOXhSTW9UM2FjUWl0Z0c2RGZibUgzOWQ3VU83M0o3dUFQWUpURG1pZGhrK0ZFOG9lbjZWUG9YRy8KNXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t"}
export LABEL_VALUE=${LABEL_VALUE:-"test"}
export NODE_SELECTOR_OS=$NODE_SELECTOR_OS

setup() {
  if [[ -z "${AZURE_CLIENT_ID}" ]] || [[ -z "${AZURE_CLIENT_SECRET}" ]]; then
    echo "Error: Azure service principal is not provided" >&2
    return 1
  fi
}

@test "install azure provider" {	
  run kubectl apply -f $PROVIDER_YAML --namespace $NAMESPACE	
  assert_success	

  kubectl wait --for=condition=Ready --timeout=120s pod -l app=csi-secrets-store-provider-azure --namespace $NAMESPACE

  AZURE_PROVIDER_POD=$(kubectl get pod --namespace $NAMESPACE -l app=csi-secrets-store-provider-azure -o jsonpath="{.items[0].metadata.name}")	

  run kubectl get pod/$AZURE_PROVIDER_POD --namespace $NAMESPACE
  assert_success
}

@test "create azure k8s secret" {
  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET}
  assert_success

  # label the node publish secret ref secret
  run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true
  assert_success
}

@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "Test rbac roles and role bindings exist" {
  run kubectl get clusterrole/secretproviderclasses-role
  assert_success

  run kubectl get clusterrole/secretproviderrotation-role
  assert_success

  run kubectl get clusterrole/secretprovidersyncing-role
  assert_success

  run kubectl get clusterrolebinding/secretproviderclasses-rolebinding
  assert_success

  run kubectl get clusterrolebinding/secretproviderrotation-rolebinding
  assert_success

  run kubectl get clusterrolebinding/secretprovidersyncing-rolebinding
  assert_success
}

@test "deploy azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl apply -f -
  
  kubectl wait --for=condition=Ready --timeout=180s pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read azure kv secret from pod" {
  wait_for_process $WAIT_TIME $SLEEP_TIME "kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME | grep '${SECRET_VALUE}'"

  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test with pod portability - read azure kv key from pod" {
  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  # https://github.com/kubernetes/kubernetes/pull/96702
  # kubectl wait --for=delete does not work on already deleted pods.
  # Instead we will start the wait before initiating the delete.
  kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline-crd &
  WAIT_PID=$!

  sleep 1
  run kubectl delete pod secrets-store-inline-crd

  # On Linux a failure to unmount the tmpfs will block the pod from being
  # deleted.
  run wait $WAIT_PID
  assert_success

  # Sleep to allow time for logs to propagate.
  sleep 10
  # save debug information to archive in case of failure
  archive_info

  # On Windows, the failed unmount calls from: https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/545
  # do not prevent the pod from being deleted. Search through the driver logs
  # for the error.
  run bash -c "kubectl logs -l app=secrets-store-csi-driver --tail -1 -c secrets-store -n kube-system | grep '^E.*failed to clean and unmount target path.*$'"
  assert_failure
}

@test "Sync with K8s secrets - create deployment" {
  envsubst < $BATS_TESTS_DIR/azure_synck8s_v1alpha1_secretproviderclass.yaml | kubectl apply -f - 

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -f -
  envsubst < $BATS_TESTS_DIR/deployment-two-synck8s-azure.yaml | kubectl apply -f -

  kubectl wait --for=condition=Ready --timeout=90s pod -l app=busybox
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  POD=$(kubectl get pod -l app=busybox -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.environment}")
  [[ "${result//$'\r'}" == "${LABEL_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.secrets-store\.csi\.k8s\.io/managed}")
  [[ "${result//$'\r'}" == "true" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 2"
  assert_success
}

@test "Sync with K8s secrets - delete deployment, check owner ref updated, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 1"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/deployment-two-synck8s-azure.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/azure_synck8s_v1alpha1_secretproviderclass.yaml
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  run kubectl create ns test-ns
  assert_success

  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET} -n test-ns
  assert_success

  # label the node publish secret ref secret
  run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true -n test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_secretproviderclass_ns.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -n test-ns -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -n test-ns -f -

  kubectl wait --for=condition=Ready --timeout=60s pod -l app=busybox -n test-ns
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=busybox -n test-ns -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret -n test-ns -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec -n test-ns $POD -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret test-ns 1"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - delete deployment, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  run kubectl create ns negative-test-ns
  assert_success

  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET} -n negative-test-ns
  assert_success

  # label the node publish secret ref secret
  run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true -n negative-test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -n negative-test-ns -f -
  sleep 5

  POD=$(kubectl get pod -l app=busybox -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/azure-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml -n negative-test-ns
  assert_success

  run kubectl delete ns negative-test-ns
  assert_success
}

@test "deploy multiple azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_multiple_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-spc-0 -o yaml | grep azure-spc-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-spc-1 -o yaml | grep azure-spc-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  envsubst < $BATS_TESTS_DIR/pod-azure-inline-volume-multiple-spc.yaml | kubectl apply -f -
  
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-multiple-crd

  run kubectl get pod/secrets-store-inline-multiple-crd
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret-0 -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_0) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-0 default 1"
  assert_success

  result=$(kubectl get secret foosecret-1 -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_1) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-1 default 1"
  assert_success
}

@test "Test auto rotation of mount contents and K8s secrets - Create deployment" {
  run kubectl create ns rotation
  assert_success

  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET} -n rotation
  assert_success

  # label the node publish secret ref secret
  run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true -n rotation
  assert_success

  run az login -u ${AZURE_CLIENT_ID} -p ${AZURE_CLIENT_SECRET} -t ${TENANT_ID} --service-principal
  assert_success

  run az keyvault secret set --vault-name ${KEYVAULT_NAME} --name ${AUTO_ROTATE_SECRET_NAME} --value secret
  assert_success

  envsubst < $BATS_TESTS_DIR/rotation/azure_synck8s_v1alpha1_secretproviderclass.yaml | kubectl apply -n rotation -f -
  envsubst < $BATS_TESTS_DIR/rotation/pod-synck8s-azure.yaml | kubectl apply -n rotation -f -

  kubectl wait -n rotation --for=condition=Ready --timeout=60s pod/secrets-store-inline-rotation

  run kubectl get pod/secrets-store-inline-rotation -n rotation
  assert_success
}

@test "Test auto rotation of mount contents and K8s secrets" {
  run kubectl cp -n rotation secrets-store-inline-rotation:/mnt/secrets-store/secretalias $BATS_TMPDIR/before_rotation
  assert_success

  result=$(cat $BATS_TMPDIR/before_rotation)
  [[ "${result//$'\r'}" == "secret" ]]
  rm $BATS_TMPDIR/before_rotation

  result=$(kubectl get secret -n rotation rotationsecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "secret" ]]

  run az keyvault secret set --vault-name ${KEYVAULT_NAME} --name ${AUTO_ROTATE_SECRET_NAME} --value rotated
  assert_success

  sleep 60

  run kubectl cp -n rotation secrets-store-inline-rotation:/mnt/secrets-store/secretalias $BATS_TMPDIR/after_rotation
  assert_success

  result=$(cat $BATS_TMPDIR/after_rotation)
  [[ "${result//$'\r'}" == "rotated" ]]
  rm $BATS_TMPDIR/after_rotation

  result=$(kubectl get secret -n rotation rotationsecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "rotated" ]]

  run az keyvault secret delete --vault-name ${KEYVAULT_NAME} --name ${AUTO_ROTATE_SECRET_NAME}
  assert_success

  run az logout
  assert_success
}

@test "Test filtered-watch-secret=false for nodePublishSecretRef" {
  local chart_dir=${HELM_CHART_DIR}
  if [[ "${chart_dir}" == "" ]]; then
    chart_dir=manifest_staging/charts/secrets-store-csi-driver
  fi
  run helm upgrade csi-secrets-store "${chart_dir}" --reuse-values --set filteredWatchSecret=false --wait --timeout=5m -v=5 --debug --namespace kube-system
  assert_success

  cmd="kubectl get crd secretproviderclasses.secrets-store.csi.x-k8s.io -o yaml | grep 'helm.sh/resource-policy: keep'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get crd secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io -o yaml | grep 'helm.sh/resource-policy: keep'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  kubectl create ns non-filtered-watch
  kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET} -n non-filtered-watch

  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_secretproviderclass.yaml | kubectl apply -n non-filtered-watch -f -
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl apply -n non-filtered-watch -f -

  kubectl wait -n non-filtered-watch --for=condition=Ready --timeout=180s pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd -n non-filtered-watch
  assert_success

  result=$(kubectl exec -n non-filtered-watch secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

teardown_file() {
  archive_provider "app=csi-secrets-store-provider-azure" || true
  archive_info || true

  #cleanup
  run kubectl delete namespace non-filtered-watch
  run kubectl delete namespace rotation
  run kubectl delete namespace test-ns
  
  run kubectl delete secret secrets-store-creds

  run kubectl delete pods secrets-store-inline-crd secrets-store-inline-multiple-crd --force --grace-period 0
}
