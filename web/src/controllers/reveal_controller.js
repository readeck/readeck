import { Controller } from "stimulus"

export default class extends Controller {
  static get targets () {
    return ["trigger", "hide", "show"]
  }
  static get values () {
    return {
      visible: Boolean,
    }
  }

  connect () {
    this.toggle()
    this.triggerTargets.forEach(e => {
      e.style.cursor = "pointer"
      e.setAttribute("data-action", `click->${this.identifier}#toggle`)
    })
  }

  toggle(evt) {
    if (evt) {
      evt.preventDefault()
    }
    this.visibleValue ? this.show() : this.hide()
  }

  show() {
    this.hideTargets.forEach(e => e.style.display = "")
    this.showTargets.forEach(e => e.style.display = "none")
    this.visibleValue = false
  }

  hide () {
    this.hideTargets.forEach(e => e.style.display = "none")
    this.showTargets.forEach(e => e.style.display = "")
    this.visibleValue = true
  }
}
