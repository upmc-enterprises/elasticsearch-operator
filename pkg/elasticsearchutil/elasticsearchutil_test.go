package elasticsearchutil

import (
	"fmt"
	"testing"
)

func TestMinMasterNodes(t *testing.T) {
	for _, v := range []struct {
		replicas int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{5, 3},
	} {

		minMasterNodes := MinMasterNodes(v.replicas)

		if minMasterNodes != v.expected {
			t.Errorf(fmt.Sprintf("Expected %d, got %d", v.expected, minMasterNodes))
		}

	}
}
