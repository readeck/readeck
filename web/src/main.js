// Fonts
import "fontsource-source-sans-pro/400.css"
import "fontsource-source-sans-pro/600.css"
import "fontsource-merriweather"

// Style
import "../style/index.sass"

// Icons
// They are all combined into a big SVG file that's included
// in the main template.
//
// Every icon is prefixed with o- (or xs- for solid icons)
// to make them easier to find in the code, should you need to.

import "@box/solid/bxs-cog.svg?o-admin"
import "@box/regular/bx-archive.svg?o-archive-off"
import "@box/solid/bxs-archive.svg?o-archive-on"
import "@box/solid/bxs-bookmarks.svg?o-bookmarks"
import "@box/regular/bx-calendar.svg?o-calendar"
import "@box/regular/bx-check-circle.svg?o-check-off"
import "@box/solid/bxs-check-circle.svg?o-check-on"
import "@box/solid/bxs-copy.svg?o-copy"
import "@box/solid/bxs-x-circle.svg?o-cross"
import "@box/regular/bx-error.svg?o-error"
import "@box/regular/bx-heart.svg?o-favorite-off"
import "@box/solid/bxs-heart.svg?o-favorite-on"
import "@box/solid/bxs-show.svg?o-hide"
import "@box/regular/bx-link.svg?o-link"
import "@box/regular/bx-log-out.svg?o-logout"
import "@box/solid/bxs-minus-circle.svg?o-minus"
import "@box/regular/bx-pen.svg?o-pen"
import "@box/solid/bxs-plus-circle.svg?o-plus"
import "@box/solid/bxs-hide.svg?o-show"
import "@box/regular/bx-trash.svg?o-trash"
import "@box/regular/bx-undo.svg?o-undo"
import "@box/regular/bx-user-circle.svg?o-user"

import "./img/spinner.svg?o-spinner"


// Launch stimulus controllers
import "regenerator-runtime/runtime"
import { Application } from "stimulus"
import { definitionsFromContext } from "stimulus/webpack-helpers"

const application = Application.start()
const context = require.context("./controllers", true, /\.js$/)
application.load(definitionsFromContext(context))

import "./lib/turbo"
