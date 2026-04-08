package zh

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// AppIDZh is the 3GPP Zh Diameter application ID (3GPP TS 29.109).
const AppIDZh = uint32(16777221)

// LoadDict registers the Zh application in the global dictionary.
// All SIP AVPs (608, 609, 610, 612, 613, 625, 626, 607) are already
// registered by the SWx dict loader, so only the application declaration
// and command mapping are needed here.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("zh: load dict: %w", err)
	}
	return nil
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777221" type="auth" name="Zh">
    <vendor id="10415" name="3GPP"/>

    <!-- Command 303 (MAR) is reused from Cx/SWx; AVPs are already registered. -->
    <command code="303" short="MAR" name="Multimedia-Auth-Request"/>

  </application>
</diameter>`
