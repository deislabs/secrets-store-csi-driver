package spcutil

import (
	"strings"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

// builds the data field of a SecretObject when syncAll is true
func BuildSecretObjectData(files map[string]string, secretObj *v1alpha1.SecretObject) {
	for key := range files {
		nested := strings.Split(key, "/")
		var renamedKey string

		if len(nested) > 0 {
			renamedKey = strings.Join(nested, "_")
		}

		if renamedKey == "" {
			secretObj.Data = append(secretObj.Data, &v1alpha1.SecretObjectData{
				ObjectName: key,
				Key:        key,
			})
			continue
		}

		secretObj.Data = append(secretObj.Data, &v1alpha1.SecretObjectData{
			ObjectName: renamedKey,
			Key:        renamedKey,
		})

	}
}
