import { Controller } from "stimulus"

export default class extends Controller {
  static get values() {
    return {
      action: String,
      method: String,
      disabled: Boolean,
    }
  }

  connect() {
    let tagName = this.element.tagName.toLowerCase()
    switch (tagName) {
    case "form":
      if (this.disabledValue) {
        this.element.setAttribute("data-action", `${this.identifier}#stopSubmit`)
      } else {
        this.conditionnalAttr("action", this.actionValue, this.hasActionValue)
        this.conditionnalAttr("method", this.methodValue, this.hasMethodValue)
      }
      break
    case "button":
      if (this.disabledValue) {
        this.element.setAttribute("data-action", `${this.identifier}#stopSubmit`)
      } else {
        this.conditionnalAttr("formaction", this.actionValue, this.hasActionValue)
        this.conditionnalAttr("formmethod", this.methodValue, this.hasMethodValue)
      }
      break
    default:
      throw new Error("turbo-form can only be used on form and button elements")
    }
  }

  conditionnalAttr(name, value, condition) {
    if (condition) {
      this.element.setAttribute(name, value)
    }
  }

  stopSubmit(evt) {
    evt.stopPropagation()
  }
}
