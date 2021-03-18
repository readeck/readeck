import { Controller } from "stimulus"
import api from "../lib/api"
import icon from "../lib/icon"

export default class extends Controller {
  static get values() {
    return {
      id: String,
      iconOn: String,
      iconOff: String,
    }
  }

  async toggle(evt) {
    if (!api.available) {
      return
    }
    evt.preventDefault()

    let data = {}
    data[this.attrName] = this.newValue
    let rsp = await api.patchJSON(`bookmarks/${this.idValue}`, data)

    this.element.value = Math.abs(rsp[this.attrName] - 1)
    this.swapIcon()
    this.element.blur()
  }

  get attrName() {
    return this.element.name
  }

  get newValue() {
    return !!parseInt(this.element.value, 10)
  }

  swapIcon() {
    let el = this.element.querySelector(".svgicon > svg")
    if (el == null) {
      return
    }
    let i = this.newValue ? this.iconOffValue : this.iconOnValue
    icon.swapIcon(el, i)
  }
}
