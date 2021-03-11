// +build ignore

package main

import (
	"log"

	"github.com/shurcooL/vfsgen"

	"github.com/readeck/readeck/internal/templates"
	"github.com/readeck/readeck/pkg/extract/fftr"
)

func main() {
	var err error
	if err = vfsgen.Generate(templates.Templates, vfsgen.Options{
		Filename:     "internal/templates/templates_vfsdata.go",
		PackageName:  "templates",
		BuildTags:    "assets",
		VariableName: "Templates",
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
