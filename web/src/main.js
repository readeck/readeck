import { Application } from "stimulus"
import { definitions } from "stimulus:./controllers"

document.body.classList.add("js")

const application = Application.start()
application.load(definitions)

import "./lib/turbo"
