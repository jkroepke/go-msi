package config

import (
	"fmt"
	"slices"
	"strings"
)

// Severity of a Diagnostic.
type Severity int

// Severities. Errors fail the build; warnings are reported.
const (
	SeverityWarning Severity = iota
	SeverityError
)

func (s Severity) String() string {
	if s == SeverityError {
		return "error"
	}

	return "warning"
}

// Diagnostic is one validation finding.
type Diagnostic struct { //nolint:govet // field order is the YAML key order
	Severity Severity
	// Path is the YAML path, such as components[0].files[1].version.
	Path    string
	Message string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s: %s: %s", d.Severity, d.Path, d.Message)
}

// Diagnostics is the result of Validate.
type Diagnostics []Diagnostic

// Err returns an error listing every error-level diagnostic, or nil.
func (ds Diagnostics) Err() error {
	var lines []string

	for _, d := range ds {
		if d.Severity == SeverityError {
			lines = append(lines, d.Path+": "+d.Message)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	return fmt.Errorf("invalid configuration:\n  %s", strings.Join(lines, "\n  "))
}

// Warnings returns the warning-level diagnostics.
func (ds Diagnostics) Warnings() Diagnostics {
	var out Diagnostics

	for _, d := range ds {
		if d.Severity == SeverityWarning {
			out = append(out, d)
		}
	}

	return out
}

type validator struct {
	scope Scope
	diags Diagnostics
}

// perUserPackage names the package in an ICE105 finding: a per-user
// package, or a dual one that can also install per user.
func (v *validator) perUserPackage() string {
	if v.scope.Dual() {
		return "a " + string(v.scope) + " package"
	}

	return "a per-user package"
}

// forbiddenFolder reports whether a component under the standard folder top
// fails ICE105 for this package's scope.
func (v *validator) forbiddenFolder(top string) bool {
	switch {
	case v.scope.Dual():
		return ice105Folders[top]
	case v.scope == ScopePerUser:
		return perMachineFolders[top]
	default:
		return false
	}
}

func (v *validator) errorf(path, format string, args ...any) {
	v.diags = append(v.diags, Diagnostic{SeverityError, path, fmt.Sprintf(format, args...)})
}

func (v *validator) warnf(path, format string, args ...any) {
	v.diags = append(v.diags, Diagnostic{SeverityWarning, path, fmt.Sprintf(format, args...)})
}

func (v *validator) required(path, value string) bool {
	if value == "" {
		v.errorf(path, "required")

		return false
	}

	return true
}

func (v *validator) oneOf(path, value string, allowed ...string) {
	if slices.Contains(allowed, value) {
		return
	}

	v.errorf(path, "%q is not one of %s", value, strings.Join(allowed, ", "))
}
