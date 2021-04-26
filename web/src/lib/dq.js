function DQ(s, root) {
  function q (s, root) {
    root = root || document
    if (typeof s === "string") {
      return [...root.querySelectorAll(s)]
    } else if (s.tagName || s instanceof DocumentFragment) {
      return [s]
    } else if (s.isDQ) {
      return s.sel()
    }

    throw TypeError(`Expected string | Element | DQ, got ${typeof src}`)
  }

  function applyTo(n, f) {
    if (n instanceof Text) {
      iter(t => f(t, n))
      return
    }
    n = DQ(n)
    iter(t => n.each((e) => f(t, e)))
  }

  let sel = q(s, root)
  let iter = sel.forEach.bind(sel)

  return {
    each (f)                { iter(f); return this },
    css (k, v)              { iter(e => e.style[k] = v); return this },
    html (h)                { iter(e => e.innerHTML = h); return this },
    text (t)                { iter(e => e.innerText = t); return this },
    addClass (...args)      { iter(e => e.classList.add(...args)); return this },
    removeClass (...args)   { iter(e => e.classList.remove(...args)); return this },
    toggleClass (...args)   { iter(e => e.classList.toggle(...args)); return this },
    attr (k, v)             { iter(e => e.setAttribute(k, v)); return this },
    attrNS (ns, k, v)       { iter(e => e.setAttributeNS(ns, k, v)); return this },
    removeAttr (k)          { iter(e => e.removeAttribute(k)); return this },
    parent ()               { iter((e, i) => { sel[i] = e.parentNode }); return this },
    remove ()               { iter(e => e.remove()); return this },
    appendTo (t)            { iter(e => t.appendChild(e)); return this },

    append (n) {
      applyTo(n, (t, e) => t.appendChild(e))
      return this
    },
    prepend (n) {
      applyTo(n, (t, e) => t.insertBefore(e, t.firstChild))
      return this
    },
    after (n) {
      applyTo(n, (t, e) => t.parentNode.insertBefore(e, t.nextSibling))
      return this
    },


    getAttr: (v) => sel[0].getAttribute(v),

    sel: () => sel,
    get: () => sel[0],
    isDQ: true,
  }
}

DQ.E = (n, ns) => DQ(ns !== undefined ? document.createElementNS(ns, n) : document.createElement(n))
DQ.T = (t) => document.createTextNode(t)

export default DQ
