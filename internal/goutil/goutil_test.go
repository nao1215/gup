package goutil

import "testing"

func TestIsAlreadyUpToDate(t *testing.T) {
	for i, test := range []struct {
		curr   string
		latest string
		expect bool
	}{
		// Regular cases
		{curr: "v1.9.0", latest: "v1.9.1", expect: false},
		{curr: "v1.9.0", latest: "v1.9.0", expect: true},
		{curr: "v1.9.1", latest: "v1.9.0", expect: true},
		// Irregular cases (untagged versions)
		{
			curr:   "v0.0.0-20220913151710-7c6e287988f3",
			latest: "v0.0.0-20210608161538-9736a4bde949",
			expect: true,
		},
		{
			curr:   "v0.0.0-20210608161538-9736a4bde949",
			latest: "v0.0.0-20220913151710-7c6e287988f3",
			expect: false,
		},
		// Compatibility between go-style semver and pure-semver
		{curr: "v1.9.0", latest: "1.9.1", expect: false},
		{curr: "v1.9.1", latest: "1.9.0", expect: true},
		{curr: "1.9.0", latest: "v1.9.1", expect: false},
		{curr: "1.9.1", latest: "v1.9.0", expect: true},
		// Issue #36
		{curr: "v1.9.1-0.20220908165354-f7355b5d2afa", latest: "v1.9.0", expect: true},
	} {
		verTmp := Version{
			Current: test.curr,
			Latest:  test.latest,
		}

		expect := test.expect
		actual := IsAlreadyUpToDate(verTmp)

		// Assert to be equal
		if expect != actual {
			t.Errorf(
				"case #%v failed. got: (\"%v\" >= \"%v\") = %v, want: %v",
				i, test.curr, test.latest, actual, expect,
			)
		}
	}
}
