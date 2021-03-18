import { Controller } from "stimulus"
import $ from "../lib/dq"
import icon from "../lib/icon"

export default class extends Controller {
  static get targets () {
    return ["field"]
  }

  static get values () {
    return {
      iconShow: String,
      iconHide: String,
      icon: String,
    }
  }

  connect() {
    // Create the button
    this.icon = icon.getIcon()

    $(this.fieldTarget).after(
      $.E("button")
        .addClass("button-clear")
        .attr("type", "button")
        .attr("data-action", `click->${this.identifier}#toggle`)
        .css("padding", "0")
        .css("marginLeft", "-2.4rem")
        .css("marginTop", "0.9rem")
        .append(this.icon),
    )

    // Set the icon
    this.iconValue = this.iconShowValue
  }

  iconValueChanged() {
    if (!this.iconValue) {
      return
    }
    icon.swapIcon(this.icon.firstChild, this.iconValue)
  }

  toggle() {
    if (this.fieldTarget.getAttribute("type") == "password") {
      this.fieldTarget.setAttribute("type", "text")
      this.iconValue = this.iconHideValue
    } else {
      this.fieldTarget.setAttribute("type", "password")
      this.iconValue = this.iconShowValue
    }
    this.fieldTarget.focus()
  }
}
