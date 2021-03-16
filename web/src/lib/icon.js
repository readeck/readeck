const iconsURL = document.querySelector('html>head>meta[name="x-icons"]').content

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
  swapIcon,
}
