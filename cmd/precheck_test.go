package cmd

import "testing"

func TestClampJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "negative", in: -1, want: 1},
		{name: "zero", in: 0, want: 1},
		{name: "one", in: 1, want: 1},
		{name: "many", in: 8, want: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := clampJobs(tt.in)
			if got != tt.want {
				t.Fatalf("clampJobs(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
