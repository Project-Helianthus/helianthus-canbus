package receiver

import (
	"strings"
	"testing"
)

func TestValidInterfaceName(t *testing.T) {
	fifteen := strings.Repeat("a", InterfaceNameMaximum)
	sixteen := strings.Repeat("a", InterfaceNameMaximum+1)
	cases := []struct {
		name string
		want bool
	}{
		{name: "can0", want: true},
		{name: fifteen, want: true},
		{name: "", want: false},
		{name: ".", want: false},
		{name: "..", want: false},
		{name: sixteen, want: false},
		{name: "can/0", want: false},
		{name: "can:0", want: false},
		{name: "can 0", want: false},
		{name: "can\x00", want: false},
	}
	for _, tc := range cases {
		if got := ValidInterfaceName(tc.name); got != tc.want {
			t.Errorf("ValidInterfaceName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
