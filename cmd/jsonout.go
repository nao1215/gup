package cmd

import (
	"encoding/json"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

// Status values reported in the machine-readable (--json) output. They form a
// stable contract for scripting and CI use, so existing values must not change.
const (
	// statusInstalled is reported by 'list' for every installed binary.
	statusInstalled = "installed"
	// statusUpToDate means the binary already matches its update channel.
	statusUpToDate = "up-to-date"
	// statusUpdateAvailable means 'check' found a newer version on the channel.
	statusUpdateAvailable = "update-available"
	// statusUpdated means 'update' successfully updated the binary.
	statusUpdated = "updated"
	// statusError means the package could not be processed; see the error field.
	statusError = "error"
)

// jsonPackage is the stable, machine-readable record emitted by --json. The
// field names are part of the public contract documented in the README.
type jsonPackage struct {
	Name               string `json:"name"`
	ImportPath         string `json:"import_path"`
	ModulePath         string `json:"module_path"`
	Channel            string `json:"channel"`
	CurrentVersion     string `json:"current_version"`
	LatestVersion      string `json:"latest_version"`
	CurrentGoVersion   string `json:"current_go_version"`
	InstalledGoVersion string `json:"installed_go_version"`
	Status             string `json:"status"`
	Error              string `json:"error,omitempty"`
}

// newJSONPackage builds a jsonPackage from package information, the resolved
// status, and an optional per-package error. Nil Version/GoVersion pointers are
// tolerated so partial-failure records can still be emitted.
func newJSONPackage(p goutil.Package, status string, err error) jsonPackage {
	rec := jsonPackage{
		Name:       p.Name,
		ImportPath: p.ImportPath,
		ModulePath: p.ModulePath,
		Channel:    string(goutil.NormalizeUpdateChannel(string(p.UpdateChannel))),
		Status:     status,
	}
	if p.Version != nil {
		rec.CurrentVersion = p.Version.Current
		rec.LatestVersion = p.Version.Latest
	}
	if p.GoVersion != nil {
		rec.CurrentGoVersion = p.GoVersion.Current
		rec.InstalledGoVersion = p.GoVersion.Latest
	}
	if err != nil {
		rec.Status = statusError
		rec.Error = err.Error()
	}
	return rec
}

// resultToJSONPackage converts an execution result into a JSON record. Error
// results are always reported with statusError regardless of the worker status.
func resultToJSONPackage(v updateResult) jsonPackage {
	return newJSONPackage(v.pkg, v.status, v.err)
}

// resultsToJSONPackages converts execution results into JSON records, preserving
// completion order.
func resultsToJSONPackages(results []updateResult) []jsonPackage {
	recs := make([]jsonPackage, 0, len(results))
	for _, v := range results {
		recs = append(recs, resultToJSONPackage(v))
	}
	return recs
}

// encodeJSONPackages writes records to stdout as an indented JSON array. A nil
// or empty slice is emitted as "[]" (never "null") so consumers always receive
// a valid JSON array.
func encodeJSONPackages(recs []jsonPackage) error {
	if recs == nil {
		recs = []jsonPackage{}
	}
	enc := json.NewEncoder(print.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(recs)
}
