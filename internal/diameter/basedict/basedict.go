// Package basedict replaces go-diameter's default dictionary with a
// controlled set of XML files.  go-diameter's built-in default dict
// includes both credit_control.xml (app 4, no vendor) and tgpp_ro_rf.xml
// (app 4, vendor 10415), producing a duplicate application-ID-4 entry
// that freeDiameter's save_remote_CE_info rejects with EINVAL.
//
// By replacing dict.Default before any sm.New() call we get a clean
// parser that contains exactly the apps we intend to advertise.
package basedict

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

//go:embed base.xml
var baseXML string

//go:embed credit_control.xml
var creditControlXML string

//go:embed tgpp_s6a.xml
var s6aXML string

// Load replaces dict.Default with a fresh parser loaded from only the
// XML files we need, avoiding duplicate application-ID entries.
// Must be called before any other dict loader and before sm.New().
func Load() error {
	fresh, err := dict.NewParser()
	if err != nil {
		return fmt.Errorf("basedict: new parser: %w", err)
	}
	for name, xml := range map[string]string{
		"base":          baseXML,
		"credit_control": creditControlXML,
		"tgpp_s6a":      s6aXML,
	} {
		if err := fresh.Load(strings.NewReader(xml)); err != nil {
			return fmt.Errorf("basedict: load %s: %w", name, err)
		}
	}
	dict.Default = fresh
	return nil
}
