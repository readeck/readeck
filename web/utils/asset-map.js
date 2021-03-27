/*
This plugin generates a json file with a mapping between an asset's
name without the hash and its real location.

It's a very naive implementation but it works for what we need.
*/
const fs = require("fs")
const path = require("path")

// Hash extractor
const rxFilename = new RegExp(/^(.+)(\.[a-z0-9]{8}\.)(.+)$/)

// We don't need to map these extensions
const excluded = [".map", ".gz", ".br", ".woff", ".woff2", ".txt"]

class AssetMap {
  constructor(options) {
    this.options = {...{
      filename: "assets.json",
    }, ...options}
  }

  apply(compiler) {
    compiler.hooks.emit.tapAsync("FileListPlugin", (compilation, callback) => {
      let res = {}
      for (let fname in compilation.assets) {
        let ext = path.extname(fname)
        if (excluded.includes(ext)) {
          continue
        }
        res[fname.replace(rxFilename, "$1.$3")] = fname
      }

      let outFile = path.join(compiler.outputPath, this.options.filename)
      fs.writeFileSync(outFile, JSON.stringify(res))
      compilation.getLogger("assetMap").info("[generated]", outFile)

      callback()
    })
  }
}

module.exports = AssetMap
