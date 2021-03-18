import { Controller } from "stimulus"

export default class extends Controller {
  static get targets () {
    return ["field"]
  }

  static get values () {
    return {
      iconShow: String,
      iconHide: String,
      icon: String,
    }
  }

  connect() {
    // Create the button element
    let el = document.createElement("button")
    el.setAttribute("type", "button")
    el.setAttribute("data-action", `click->${this.identifier}#toggle`)
    el.setAttribute("class", "button-clear")

    // Add the icon and give it some space
    el.innerHTML = this.icon()
    el.style.padding = "0"
    el.style.marginLeft = "-2.4rem"
    el.style.marginTop = "0.9rem"
    this.fieldTarget.parentNode.insertBefore(el, this.fieldTarget.nextSibling)

    this.fieldTarget.style.paddingRight = "2.4rem"
    this.fieldTarget.style.width = "calc(100% - 0.8rem)"

    // Set the icon url
    this.iconValue = this.iconShowValue
  }

  iconValueChanged() {
    let e = this.element.querySelector(".svgicon>svg>use")
    if (e === null || !this.iconValue) {
      return
    }
    e.setAttribute("xlink:href", this.iconValue)
  }

  toggle() {
    if (this.fieldTarget.getAttribute("type") == "password") {
      this.fieldTarget.setAttribute("type", "text")
      this.iconValue = this.iconHideValue
    } else {
      this.fieldTarget.setAttribute("type", "password")
      this.iconValue = this.iconShowValue
    }
    this.fieldTarget.focus()
  }

  icon() {
    return '<span class="svgicon"><svg xmlns="http://www.w3.org/2000/svg" viewbox="0 0 100 100" width="16"><use xlink:href=""></use></svg></span>'
  }
}
