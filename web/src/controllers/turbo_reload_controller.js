import { Controller } from "stimulus"

// This controller listens for turbo:submit-end events
// and reload the frame with the ID given by data-turbo-reload-frame-id-value
// attribute by reloading the current page when the form is submited.
// It obviously applies only to form elements.
export default class extends Controller {
  static get values () {
    return {
      frameId: String,
    }
  }

  connect () {
    document.addEventListener("turbo:submit-end", this.reloadFrame.bind(this))
  }

  disconnect () {
    document.removeEventListener("turbo:submit-end", this.reloadFrame.bind(this))
  }

  async reloadFrame(evt) {
    if (evt.target != this.element) {
      return
    }
    let el = document.getElementById(this.frameIdValue)
    if (!el) {
      return
    }

    el.src = document.location.href
    await el.loaded
    el.src = null
  }
}
