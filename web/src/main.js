// Fonts
import "fontsource-source-sans-pro"
import "fontsource-merriweather"

// Style
import "../style/index.sass"

// Icons
// They are all combined into a big SVG file that's included
// in the main template.
import "@box/regular/bx-archive.svg"
import "@box/solid/bxs-archive.svg"
import "@box/solid/bxs-bookmarks.svg"
import "@box/regular/bx-check-circle.svg"
import "@box/solid/bxs-check-circle.svg"
import "@box/solid/bxs-copy.svg"
import "@box/regular/bx-pen.svg"
import "@box/regular/bx-calendar.svg"
import "@box/regular/bx-heart.svg"
import "@box/solid/bxs-heart.svg"
import "@box/regular/bx-link.svg"
import "@box/regular/bx-log-out.svg"
import "@box/regular/bx-trash.svg"
import "@box/regular/bx-user-circle.svg"
import "@box/solid/bxs-show.svg"
import "@box/solid/bxs-hide.svg"


// Launch stimulus controllers
import "regenerator-runtime/runtime"
import { Application } from "stimulus"
import { definitionsFromContext } from "stimulus/webpack-helpers"

const application = Application.start()
const context = require.context("./controllers", true, /\.js$/)
application.load(definitionsFromContext(context))
