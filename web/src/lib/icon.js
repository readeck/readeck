import $ from "./dq"

const iconsURL = document.querySelector('html>head>meta[name="x-icons"]').content
const svgNS = "http://www.w3.org/2000/svg"

// getIcon returns an element with an svg icon for the given name.
function getIcon(name) {
  return $.E("span")
    .addClass("svgicon")
    .append(
      $.E("svg", svgNS)
        .attrNS(null, "viewbox", "0 0 100 100")
        .attrNS(null, "width", "16")
        .append($.E("use", svgNS)
          .attrNS(null, "href", `${iconsURL}#${name || ""}`),
        ),
    )
    .get()
}

// swapIcon changes the href of the first <use> tag in the
// given svg element.
function swapIcon(el, name) {
  let use = [...el.children].find(e => e.nodeName == "use")
  if (use === null) {
    return
  }

  use.setAttribute("href", `${iconsURL}#${name}`)
}

export default {
  getIcon,
  swapIcon,
}
