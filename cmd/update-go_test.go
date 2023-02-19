package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_golangTarballChecksums(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "get golang tarball checksums",
			want: map[string]string{
				"go1.20.1.darwin-amd64.tar.gz":  "a300a45e801ab459f3008aae5bb9efbe9a6de9bcd12388f5ca9bbd14f70236de",
				"go1.20.1.darwin-arm64.tar.gz":  "f1a8e06c7f1ba1c008313577f3f58132eb166a41ceb95ce6e9af30bc5a3efca4",
				"go1.20.1.linux-386.tar.gz":     "3a7345036ebd92455b653e4b4f6aaf4f7e1f91f4ced33b23d7059159cec5f4d7",
				"go1.20.1.linux-amd64.tar.gz":   "000a5b1fca4f75895f78befeb2eecf10bfff3c428597f3f1e69133b63b911b02",
				"go1.20.1.linux-arm64.tar.gz":   "5e5e2926733595e6f3c5b5ad1089afac11c1490351855e87849d0e7702b1ec2e",
				"go1.20.1.linux-armv6l.tar.gz":  "e4edc05558ab3657ba3dddb909209463cee38df9c1996893dd08cde274915003",
				"go1.20.1.freebsd-386.tar.gz":   "57d80349dc4fbf692f8cd85a5971f97841aedafcf211e367e59d3ae812292660",
				"go1.20.1.freebsd-amd64.tar.gz": "6e124d54d5850a15fdb15754f782986f06af23c5ddb6690849417b9c74f05f98",
				"go1.20.1.linux-ppc64le.tar.gz": "85cfd4b89b48c94030783b6e9e619e35557862358b846064636361421d0b0c52",
				"go1.20.1.linux-s390x.tar.gz":   "ba3a14381ed4538216dec3ea72b35731750597edd851cece1eb120edf7d60149",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := golangTarballChecksums()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
