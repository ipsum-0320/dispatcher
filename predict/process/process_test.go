package process

import (
	"fmt"
	"testing"
)

func Test_Process(t *testing.T) {

	var strs []string
	for i := 0; i < 100; i++ {
		strs = append(strs, fmt.Sprintf("siteId-%v", i))
	}

	for i := 0; i < 10; i++ {
		go Process("zoneId", strs)
	}
}
