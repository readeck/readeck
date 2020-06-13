// +build ignore

package main

import (
	"log"

	"github.com/shurcooL/vfsgen"

	"github.com/readeck/readeck/pkg/assets"
	"github.com/readeck/readeck/pkg/extract/fftr"
)

func main() {
	var err error
	if err = vfsgen.Generate(assets.Assets, vfsgen.Options{
		Filename:     "pkg/assets/assets_vfsdata.go",
		PackageName:  "assets",
		BuildTags:    "assets",
		VariableName: "Assets",
	}); err != nil {
		log.Fatalln(err)
	}

	if err = vfsgen.Generate(fftr.SiteConfigFolder, vfsgen.Options{
		Filename:     "pkg/extract/fftr/siteconfig_vfsdata.go",
		PackageName:  "fftr",
		BuildTags:    "assets",
		VariableName: "SiteConfigFolder",
	}); err != nil {
		log.Fatalln(err)
	}
}
