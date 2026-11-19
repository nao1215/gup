// Package e2e classifies gup's end-to-end specs by operating system.
//
// Deprecated: it is part of gup's own test harness. The implementation moved
// to an internal package, and this package remains only so that v1 importers
// keep building; it will be removed in v2.
package e2e

import "github.com/nao1215/gup/e2e/internal/osmatrix"

// MatrixFileName is the committed classification table, relative to the e2e
// directory.
//
// Deprecated: it will be removed in v2.
const MatrixFileName = osmatrix.MatrixFileName

// SpecDirName holds the atago specs, relative to the e2e directory.
//
// Deprecated: it will be removed in v2.
const SpecDirName = osmatrix.SpecDirName

// SupportedOS lists the operating systems the classification covers.
//
// Deprecated: it will be removed in v2.
var SupportedOS = osmatrix.SupportedOS //nolint:gochecknoglobals // kept for v1 compatibility

// Classification records which operating systems one spec file runs on, and
// why it does not run on the others.
//
// Deprecated: it will be removed in v2.
type Classification = osmatrix.Classification

// LoadMatrix reads the classification table in dir.
//
// Deprecated: it will be removed in v2.
func LoadMatrix(dir string) ([]Classification, error) { return osmatrix.LoadMatrix(dir) }

// SpecFiles lists the atago spec files under dir.
//
// Deprecated: it will be removed in v2.
func SpecFiles(dir string) ([]string, error) { return osmatrix.SpecFiles(dir) }

// TargetsFor returns the spec files under dir that run on goos.
//
// Deprecated: it will be removed in v2.
func TargetsFor(dir, goos string) ([]string, error) { return osmatrix.TargetsFor(dir, goos) }
