package k8sutil

import (
	"fmt"
	"testing"
)

func TestGetESURL(t *testing.T) {

	for _, v := range []struct {
		host      string
		expected  string
		enableSSL bool
	}{
		{"es-ssl", "https://es-ssl:9200", true},
		{"es-bla", "http://es-bla:9200", false},
	} {

		esURL := GetESURL(v.host, v.enableSSL)

		if esURL != v.expected {
			t.Errorf(fmt.Sprintf("Expected %s, got %s", v.expected, esURL))
		}

	}
}