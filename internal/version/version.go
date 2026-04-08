// Package version holds the application and API version strings.
//
// AppVersion is set at build time via -ldflags:
//
//	go build -ldflags "-X github.com/svinson1121/vectorcore-hss/internal/version.AppVersion=1.2.3" ./cmd/hss
//
// APIVersion must be bumped manually whenever the REST API surface changes
// (new endpoints, removed fields, changed behaviour).
package version

import "time"

// AppVersion is the binary release version. Overridden at build time.
var AppVersion = "0.2.3A"

// APIVersion is the REST API contract version. Increment on breaking or
// additive API changes so clients can detect incompatibilities.
const APIVersion = "1.0.0"

// StartTime records when the process started, used to compute uptime.
var StartTime = time.Now()
