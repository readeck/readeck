// +build !assets

package fftr

import (
	"net/http"
)

// SiteConfigFolder is the configuration folder with
// site config files.
var SiteConfigFolder = http.Dir("site-config")
