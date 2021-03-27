import { Controller } from "stimulus"

// This controller reload a given turbo-frame at a given interval
// until it find a target named "loaded" in its content.
// This replaces a meta refresh of the full page and can be used on
// several frames on the same page.
export default class extends Controller {
  static get values () {
    return {
      // A CSS selector that triggers the refresh when present
      on: String,
      // The page source to load, uses window.location if none
      src: String,
      // Refresh every given seconds
      interval: Number,
    }
  }

  connect () {
    if (!this.onValue) {
      throw new Error(`you must set data-${this.identifier}-on-value on the component`)
    }

    if (!this.hasSrcValue) {
      this.srcValue = window.location.href
    }

    // We need this to check the selector on every mutation
    this.timeout = null

    if (!this.isLoaded()) {
      this.check()
    }

    this.observer = new MutationObserver(() => this.check())
    this.observer.observe(this.element, { attributes: true, childList: true, subtree: true })
  }

  disconnect () {
    this.observer.disconnect()
  }

  async check () {
    await this.element.loaded
    if (this.isLoaded()) {
      return
    }
    if (this.timeout !== null) {
      return
    }

    this.timeout = window.setTimeout(async () => {
      this.element.src = this.srcValue
      await this.element.loaded

      this.element.src = null
      this.timeout = null
    }, this.intervalValue*1000)
  }

  isLoaded () {
    return this.element.querySelector(this.onValue) === null
  }
}
