import { Controller } from "stimulus"

export default class extends Controller {
  static get values() {
    return {
      action: String,
      method: String,
    }
  }

  connect() {
    let tagName = this.element.tagName.toLowerCase()

    switch (tagName) {
    case "form":
      this.element.setAttribute("action", this.actionValue)
      this.element.setAttribute("method", this.methodValue)
      break
    case "button":
      this.element.setAttribute("formaction", this.actionValue)
      this.element.setAttribute("formmethod", this.methodValue)
      break
    default:
      throw new Error("turbo-form can only be used on form and button elements")
    }
  }
}
