package cmd

import "context"

// testDeps returns a dependencies value whose operations are harmless stubs: the
// version lookups return an empty version and the installs are no-ops. A test
// overrides only the fields it exercises and passes the value in directly, so it
// owns its dependencies instead of mutating package globals.
func testDeps() dependencies {
	return dependencies{
		getLatestVer:        func(context.Context, string) (string, error) { return "", nil },
		getVerByRef:         func(context.Context, string, string) (string, error) { return "", nil },
		installLatest:       func(context.Context, string) error { return nil },
		installMainOrMaster: func(context.Context, string) error { return nil },
		installByVersion:    func(context.Context, string, string) error { return nil },
	}
}

// stubUpdateDeps returns dependencies that make every package look outdated
// (latest == v9.9.9) with no-op installs, so an update/check run reaches the
// install path without performing real installs or network lookups. It replaces
// the old helper_stubUpdateOps global-swap helper.
func stubUpdateDeps() dependencies {
	d := testDeps()
	d.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	// The channel-aware skip/update decision resolves @main/@master versions
	// through this ref lookup, so stub it alongside the @latest lookup.
	d.getVerByRef = func(context.Context, string, string) (string, error) { return testVersionNine, nil }
	return d
}
