import { Controller } from "stimulus"
import icon from "../lib/icon"

export default class extends Controller {
  static get targets () {
    return ["label", "content"]
  }

  connect () {
    let el = document.createElement("button")
    el.setAttribute("type", "button")
    el.setAttribute("class", "button-clear")
    el.setAttribute("data-action", `${this.identifier}#copy`)
    el.appendChild(icon.getIcon("bxs-copy"))

    this.labelTarget.appendChild(el)
  }

  async copy() {
    await navigator.clipboard.writeText(this.contentTarget.value)
  }
}
