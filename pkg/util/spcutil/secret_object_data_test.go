package spcutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

func TestBuildSecretObjectData(t *testing.T) {
	files := map[string]string{}

	files["username"] = "a test user"
	files["password"] = "a test password"
	files["nested/username"] = "a test user"

	secretObj := &v1alpha1.SecretObject{
		SecretName: "test-secret",
		Type:       "Opaque",
		SyncAll:    true,
	}

	BuildSecretObjectData(files, secretObj)

	expected := &v1alpha1.SecretObject{
		SecretName: "test-secret",
		Type:       "Opaque",
		SyncAll:    true,
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: "username",
				Key:        "username",
			},
			{
				ObjectName: "password",
				Key:        "password",
			},
			{
				ObjectName: "nested/username",
				Key:        "nested_username",
			},
		},
	}

	assert.Equal(t, expected, secretObj)
}
