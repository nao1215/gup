package e2e

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/e2e/internal/osmatrix"
)

// TestDeprecatedAPI_returnsWhatOsmatrixReturns checks that the deprecated v1
// entry points still answer exactly as the internal implementation does.
func TestDeprecatedAPI_returnsWhatOsmatrixReturns(t *testing.T) {
	t.Parallel()

	gotMatrix, err := LoadMatrix(".")
	if err != nil {
		t.Fatal(err)
	}
	wantMatrix, err := osmatrix.LoadMatrix(".")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantMatrix, gotMatrix); diff != "" {
		t.Errorf("LoadMatrix differs (-osmatrix +e2e):\n%s", diff)
	}

	gotSpecs, err := SpecFiles(".")
	if err != nil {
		t.Fatal(err)
	}
	wantSpecs, err := osmatrix.SpecFiles(".")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantSpecs, gotSpecs); diff != "" {
		t.Errorf("SpecFiles differs (-osmatrix +e2e):\n%s", diff)
	}

	for _, goos := range SupportedOS {
		got, err := TargetsFor(".", goos)
		if err != nil {
			t.Fatal(err)
		}
		want, err := osmatrix.TargetsFor(".", goos)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TargetsFor(%q) differs (-osmatrix +e2e):\n%s", goos, diff)
		}
	}
}
