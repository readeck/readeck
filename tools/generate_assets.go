// +build ignore

package main

import (
	"log"

	"github.com/shurcooL/vfsgen"

	"github.com/readeck/readeck/pkg/extract/fftr"
)

func main() {
	var err error
	if err = vfsgen.Generate(fftr.SiteConfigFolder, vfsgen.Options{
		Filename:     "pkg/extract/fftr/siteconfig_vfsdata.go",
		PackageName:  "fftr",
		BuildTags:    "assets",
		VariableName: "SiteConfigFolder",
	}); err != nil {
		log.Fatalln(err)
	}
}
