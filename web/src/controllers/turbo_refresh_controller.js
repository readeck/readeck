import { Controller } from "stimulus"

// This controller reload a given turbo-frame at a given interval
// until it find a target named "loaded" in its content.
// This replaces a meta refresh of the full page and can be used on
// several frames on the same page.
export default class extends Controller {
  static get targets () {
    return ["loaded"]
  }
  static get values () {
    return {
      src: String,
      interval: Number,
    }
  }

  connect() {
    if (this.isLoaded()) {
      return
    }

    if (!this.hasSrcValue) {
      this.srcValue = window.location.href
    }

    if (!this.hasIntervalValue) {
      this.intervalValue = 5
    }

    let interval = window.setInterval(async () => {
      this.element.src = this.srcValue
      await this.element.loaded
      this.element.src = null

      if (this.isLoaded()) {
        window.clearInterval(interval)
      }

    }, this.intervalValue*1000)
  }

  isLoaded () {
    return this.loadedTargets.length > 0
  }
}
