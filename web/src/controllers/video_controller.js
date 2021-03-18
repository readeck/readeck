import { Controller } from "stimulus"
import $ from "../lib/dq"

export default class extends Controller {
  static get values () {
    return {
      embed: String,
    }
  }

  connect() {
    if (!this.embedValue) {
      return
    }

    this.tpl = $.E("template").html(this.embedValue.trim())
    this.ifr = $("iframe", this.tpl.get().content)
    this.ifr.attr("sandbox", "allow-scripts allow-same-origin")

    let w = parseInt(this.ifr.getAttr("width")) || 0
    let h = parseInt(this.ifr.getAttr("height")) || 0
    if (w > 0 && h > 0) {
      this.element.style.paddingTop = `${100 * h / w}%`
    }

    this.playBtn = $.E("div")
      .addClass("play-button")
      .attr("data-action", `click->${this.identifier}#play`)
      .appendTo(this.element)
  }

  play() {
    this.playBtn.remove()
    $("img", this.element).remove()
    this.ifr.appendTo(this.element)
  }
};
