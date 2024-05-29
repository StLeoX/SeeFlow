package tracer

import (
	"fmt"
	"testing"
)

func TestFoo(t *testing.T) {
	endpoints := make([]*Endpoint, 0)
	fetchEndpoints(endpoints)
	for _, endpoint := range endpoints {
		fmt.Printf("+%v\n", endpoint)
	}
}
