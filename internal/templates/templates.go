// +build !assets

package templates

import "net/http"

// Templates contains projet templates
var Templates http.FileSystem = http.Dir("web/templates")
