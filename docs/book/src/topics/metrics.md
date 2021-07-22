# Metrics provided by Secrets Store CSI Driver

The Secrets Store CSI Driver uses [opentelemetry](https://opentelemetry.io/) for reporting metrics. This project is under [active development](https://github.com/open-telemetry/opentelemetry-go#release-schedule)

Prometheus is the only exporter that's currently supported with the driver.

## List of metrics provided by the driver

| Metric                           | Description                                                               | Tags                                                                                                                                                                                                                 |
| -------------------------------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| total_node_publish               | Total number of successful volume mount requests                          | `os_type=<runtime os>`<br>`provider=<provider name>`                                                                                                                                                                 |
| total_node_unpublish             | Total number of successful volume unmount requests                        | `os_type=<runtime os>`                                                                                                                                                                                               |
| total_node_publish_error         | Total number of errors with volume mount requests                         | `os_type=<runtime os>`<br>`provider=<provider name>`<br>`error_type=<error code>`                                                                                                                                    |
| total_node_unpublish_error       | Total number of errors with volume unmount requests                       | `os_type=<runtime os>`                                                                                                                                                                                               |
| total_sync_k8s_secret            | Total number of k8s secrets synced                                        | `os_type=<runtime os>`<br>`provider=<provider name>`                                                                                                                                                                 |
| sync_k8s_secret_duration_sec     | Distribution of how long it took to sync k8s secret                       | `os_type=<runtime os>`                                                                                                                                                                                               |
| total_rotation_reconcile         | Total number of rotation reconciles                                       | `os_type=<runtime os>`<br>`rotated=<true or false>`                                                                                                                                                                  |
| total_rotation_reconcile_error   | Total number of rotation reconciles with error                            | `os_type=<runtime os>`<br>`rotated=<true or false>`<br>`error_type=<error code>`                                                                                                                                     |
| rotation_reconcile_duration_sec  | Distribution of how long it took to rotate secrets-store content for pods | `os_type=<runtime os>`                                                                                                                                                                                               |
| kube_secretproviderclass_info    | Information about SecretProviderClass                                     | `os_type=<runtime os>`<br>`secretproviderclass=<secretproviderclass name>`<br>`namespace=<namespace>`                                                                                                                |
| kube_secretproviderclass_type    | Type about SecretProviderClass                                            | `os_type=<runtime os>`<br>`secretproviderclass=<secretproviderclass name>`<br>`namespace=<namespace>`<br>`secret_name=<secret object name>`<br>`secret_type_NAME=<secret object type>`<br>`provider=<provider name>` |
| kube_secretproviderclass_labels  | Kubernetes labels converted to Prometheus labels                          | `os_type=<runtime os>`<br>`secretproviderclass=<secretproviderclass name>`<br>`namespace=<namespace>`<br>`label_LABEL=<value>`                                                                                       |
| kube_secretproviderclass_created | Unix creation timestamp                                                   | `os_type=<runtime os>`<br>`secretproviderclass=<secretproviderclass name>`<br>`namespace=<namespace>`                                                                                                                |

### Sample Metrics output

```shell
# HELP kube_secretproviderclass_created Unix creation timestamp
# TYPE kube_secretproviderclass_created gauge
kube_secretproviderclass_created{namespace="default",os_type="linux",secretproviderclass="vault-foo"} 1.6260898e+09
kube_secretproviderclass_created{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync"} 1.626089808e+09
kube_secretproviderclass_created{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-0"} 1.626089818e+09
kube_secretproviderclass_created{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-1"} 1.626089818e+09
kube_secretproviderclass_created{namespace="test-ns",os_type="linux",secretproviderclass="vault-foo-sync"} 1.626089808e+09
# HELP kube_secretproviderclass_info Information about SecretProviderClass
# TYPE kube_secretproviderclass_info gauge
kube_secretproviderclass_info{namespace="default",os_type="linux",secretproviderclass="vault-foo"} 1
kube_secretproviderclass_info{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync"} 1
kube_secretproviderclass_info{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-0"} 1
kube_secretproviderclass_info{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-1"} 1
kube_secretproviderclass_info{namespace="test-ns",os_type="linux",secretproviderclass="vault-foo-sync"} 1
# HELP kube_secretproviderclass_labels Kubernetes labels converted to OpenTelemetry labels
# TYPE kube_secretproviderclass_labels gauge
kube_secretproviderclass_labels{namespace="default",os_type="linux",secretproviderclass="vault-foo"} 1
kube_secretproviderclass_labels{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-0"} 1
kube_secretproviderclass_labels{namespace="default",os_type="linux",secretproviderclass="vault-foo-sync-1"} 1
kube_secretproviderclass_labels{namespace="test-ns",os_type="linux",secretproviderclass="vault-foo-sync"} 1
kube_secretproviderclass_labels{label_a="b",label_c="d",namespace="default",os_type="linux",secretproviderclass="vault-foo-sync"} 1
# HELP kube_secretproviderclass_type Type about SecretProviderClass
# TYPE kube_secretproviderclass_type gauge
kube_secretproviderclass_type{namespace="default",os_type="linux",provider="vault",secretproviderclass="vault-foo"} 1
kube_secretproviderclass_type{namespace="default",os_type="linux",provider="invalidprovider",secret_name="foosecret",secret_type_foosecret="Opaque",secretproviderclass="vault-foo-sync"} 1
kube_secretproviderclass_type{namespace="default",os_type="linux",provider="vault",secret_name="foosecret-0",secret_type_foosecret_0="Opaque",secretproviderclass="vault-foo-sync-0"} 1
kube_secretproviderclass_type{namespace="default",os_type="linux",provider="vault",secret_name="foosecret-1",secret_type_foosecret_1="Opaque",secretproviderclass="vault-foo-sync-1"} 1
kube_secretproviderclass_type{namespace="test-ns",os_type="linux",provider="vault",secret_name="foosecret",secret_type_foosecret="Opaque",secretproviderclass="vault-foo-sync"} 1
# HELP sync_k8s_secret_duration_sec Distribution of how long it took to sync k8s secret
# TYPE sync_k8s_secret_duration_sec histogram
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.1"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.2"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.3"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.4"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="1"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="1.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="2"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="2.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="3"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="10"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="15"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="30"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="+Inf"} 1
sync_k8s_secret_duration_sec_sum{os_type="linux"} 0.3115892
sync_k8s_secret_duration_sec_count{os_type="linux"} 1
# HELP total_node_publish Total number of node publish calls
# TYPE total_node_publish counter
total_node_publish{os_type="linux",provider="azure"} 1
# HELP total_node_publish_error Total number of node publish calls with error
# TYPE total_node_publish_error counter
total_node_publish_error{error_type="ProviderBinaryNotFound",os_type="linux",provider="azure"} 2
total_node_publish_error{error_type="SecretProviderClassNotFound",os_type="linux",provider=""} 4
# HELP total_node_unpublish Total number of node unpublish calls
# TYPE total_node_unpublish counter
total_node_unpublish{os_type="linux"} 1
# HELP total_sync_k8s_secret Total number of k8s secrets synced
# TYPE total_sync_k8s_secret counter
total_sync_k8s_secret{os_type="linux",provider="azure"} 1
```
