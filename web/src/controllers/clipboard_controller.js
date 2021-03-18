import { Controller } from "stimulus"

export default class extends Controller {
  static get values () {
    return {
      jwt: String,
    }
  }

  async copy() {
    await navigator.clipboard.writeText(this.jwtValue)
  }
}
