import { Application } from "stimulus"
import { definitions } from "stimulus:./controllers"

const application = Application.start()
application.load(definitions)

import "./lib/turbo"
