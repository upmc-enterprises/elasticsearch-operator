package k8sutil

import (
	"fmt"
	"testing"

	v1 "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestGetESURL(t *testing.T) {

	for _, v := range []struct {
		host     string
		expected string
		useSSL   bool
	}{
		{"es-ssl", "https://es-ssl:9200", true},
		{"es-bla", "http://es-bla:9200", false},
	} {

		esURL := GetESURL(v.host, &v.useSSL)

		if esURL != v.expected {
			t.Errorf(fmt.Sprintf("Expected %s, got %s", v.expected, esURL))
		}

	}
}

func TestSSLCertConfig(t *testing.T) {

	memoryCPU := v1.MemoryCPU{
		Memory: "128Mi",
		CPU:    "100m",
	}
	resources := v1.Resources{
		Requests: memoryCPU,
		Limits:   memoryCPU,
	}
	clusterName := "test"
	useSSL := false
	nodeSelector := make(map[string]string)
	tolerations := []corev1.Toleration{}
	annotations := make(map[string]string)
	statefulSet := buildStatefulSet("test", clusterName, "master", "foo/image", "test", "1G", "",
		"", "", "", "", "", nil, &useSSL, resources, nil, "", nodeSelector, tolerations, annotations)

	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name == fmt.Sprintf("%s-%s", secretName, clusterName) {
			t.Errorf("Found volume for certificates, was not expecting it since useSSL is false")
		}
	}

	useSSL = true
	statefulSet = buildStatefulSet("test", clusterName, "master", "foo/image", "test", "1G", "",
		"", "", "", "", "", nil, &useSSL, resources, nil, "", nodeSelector, tolerations, annotations)

	found := false
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name == fmt.Sprintf("%s-%s", secretName, clusterName) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Volume for certificates not found, was expecting it since useSSL is true")
	}
}
