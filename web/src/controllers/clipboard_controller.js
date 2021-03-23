import { Controller } from "stimulus"
import $ from "../lib/dq"
import icon from "../lib/icon"

export default class extends Controller {
  static get targets () {
    return ["label", "content"]
  }

  connect () {
    $.E("button")
      .addClass("button-clear")
      .attr("type", "button")
      .attr("data-action", `${this.identifier}#copy`)
      .append(icon.getIcon("o-copy"))
      .appendTo(this.labelTarget)
  }

  async copy() {
    await navigator.clipboard.writeText(this.contentTarget.value)
  }
}
