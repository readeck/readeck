const iconsURL = document.querySelector('html>head>meta[name="x-icons"]').content


// getIcon returns an element with an svg icon for the given name.
function getIcon(name) {
  let e = document.createElement("span")
  e.setAttribute("class", "svgicon")
  let s = document.createElementNS("http://www.w3.org/2000/svg", "svg")
  s.setAttributeNS("http://www.w3.org/2000/svg", "viewbox", "0 0 100 100")
  s.setAttributeNS("http://www.w3.org/2000/svg", "width", "16")
  let u = document.createElementNS("http://www.w3.org/2000/svg", "use")

  e.appendChild(s)
  s.appendChild(u)

  u.setAttributeNS(null, "href", `${iconsURL}#${name}`)
  return e
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
