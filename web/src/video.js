import * as rxjs from "rxjs"

(() => {
  const el = document.querySelector("div.video-player")
  if (!el) {
    return
  }

  const embed = el.dataset.embed
  if (!embed) {
    return
  }

  const node = document.createElement("template")
  let w, h = 0
  node.innerHTML = embed.trim()
  node.content.querySelectorAll("iframe").forEach(n => {
    n.setAttribute("sandbox", "allow-scripts allow-same-origin")
    w = parseInt(n.getAttribute("width")) || 0
    h = parseInt(n.getAttribute("height")) || 0
  })

  if (w > 0 && h > 0) {
    el.style.paddingTop = `${100 * h / w}%`
  }

  const playBtn = document.createElement("div")
  playBtn.classList.add("play-button")
  el.appendChild(playBtn)

  rxjs.fromEvent(playBtn, "click").subscribe(x => {
    playBtn.parentNode.removeChild(playBtn)
    el.querySelectorAll("img").forEach(e => {
      e.parentNode.removeChild(e)
    })
    el.appendChild(node.content)
  })
})()
