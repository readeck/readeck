import { Controller } from "stimulus"

export default class extends Controller {
  static values = {jwt: String}

  async copy() {
    await navigator.clipboard.writeText(this.jwtValue)
  }
};
