package cmd

import (
	"context"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/vercache"
)

// dependencies bundles the go-toolchain operations the update and check flows
// rely on: the online version lookups and the per-channel install variants.
//
// Threading these through the operation flow as a value (rather than reading
// package-level globals deep inside the business logic) lets the runner take its
// I/O-bound collaborators explicitly, and lets a test inject fakes by building a
// dependencies value instead of mutating shared globals. defaultDependencies is
// the single place that names the real goutil operations, so the rest of the
// package depends only on the injected value and tests inject directly and run
// in parallel.
type dependencies struct {
	getLatestVer        func(ctx context.Context, modulePath string) (string, error)
	getVerByRef         func(ctx context.Context, modulePath, ref string) (string, error)
	installLatest       func(ctx context.Context, importPath string) error
	installMainOrMaster func(ctx context.Context, importPath string) error
	installByVersion    func(ctx context.Context, importPath, version string) error
}

// defaultDependencies wires the real goutil operations used in production. It is
// the one place that names them, so the business logic (newVerCache,
// updatePinned, installWithSelectedVersion) and the command entry points depend
// only on the injected value.
func defaultDependencies() dependencies {
	return dependencies{
		getLatestVer:        goutil.GetLatestVerWithContext,
		getVerByRef:         goutil.GetVerWithContext,
		installLatest:       goutil.InstallLatestWithContext,
		installMainOrMaster: goutil.InstallMainOrMasterWithContext,
		installByVersion:    goutil.InstallWithContext,
	}
}

// newVerCache builds the per-(module,channel) version cache used by update and
// check, wiring the injected lookup operations into vercache's channel policy.
// The seams are read through the injected funcs so a test that supplies its own
// dependencies controls the resolved versions without touching globals.
func (d dependencies) newVerCache() *vercache.Cache {
	return vercache.New(vercache.ChannelResolver(
		func(ctx context.Context, modulePath string) (string, error) {
			return d.getLatestVer(ctx, modulePath)
		},
		func(ctx context.Context, modulePath, ref string) (string, error) {
			return d.getVerByRef(ctx, modulePath, ref)
		},
	))
}
