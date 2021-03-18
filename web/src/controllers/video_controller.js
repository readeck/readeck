import { Controller } from "stimulus"

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

    this.tpl = document.createElement("template")
    let w, h = 0
    this.tpl.innerHTML = this.embedValue.trim()
    this.tpl.content.querySelectorAll("iframe").forEach(n => {
      n.setAttribute("sandbox", "allow-scripts allow-same-origin")
      w = parseInt(n.getAttribute("width")) || 0
      h = parseInt(n.getAttribute("height")) || 0
    })

    if (w > 0 && h > 0) {
      this.element.style.paddingTop = `${100 * h / w}%`
    }

    this.playBtn = document.createElement("div")
    this.playBtn.classList.add("play-button")
    this.playBtn.setAttribute("data-action", "click->video#play")
    this.element.appendChild(this.playBtn)
  }

  play() {
    this.playBtn.parentNode.removeChild(this.playBtn)
    this.element.querySelectorAll("img").forEach(e => {
      e.parentNode.removeChild(e)
    })
    this.element.appendChild(this.tpl.content)
  }
};
