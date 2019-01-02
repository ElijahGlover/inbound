package helpers_test

import (
	"testing"

	"github.com/elijahglover/inbound/internal/helpers"
)

func Test_Hostname_Extract(t *testing.T) {
	input := "localhost:8080"
	expected := "localhost"

	actual := helpers.ExtractHostname(input)
	if actual != expected {
		t.Fatalf("unexpected output %s %s", actual, expected)
	}
}
