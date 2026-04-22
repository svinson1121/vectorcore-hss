// Package version holds the application and API version strings.
//
// AppVersion and APIVersion are typically set by the Makefile via -ldflags:
//
//	make build
//
// They keep fallback defaults here so direct `go build` still produces a usable
// binary outside the Makefile.
package version

import "time"

// AppVersion is the binary release version. Overridden at build time.
var AppVersion = "0.3.0B"

// APIVersion is the REST API contract version. Increment on breaking or
// additive API changes so clients can detect incompatibilities. Overridden at
// build time when built through the Makefile.
var APIVersion = "1.0.0"

// StartTime records when the process started, used to compute uptime.
var StartTime = time.Now()
