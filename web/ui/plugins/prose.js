const plugin = require("tailwindcss/plugin")
const defaultTheme = require("tailwindcss/defaultTheme")

function verticalFlow(lineHeight, fontSize) {
  const minLH = 0.85
  const res = {}

  // Lines covered
  let covers = Math.ceil(fontSize / lineHeight)

  // We could use one line less
  if (covers > 1 && (covers - 1) * lineHeight / fontSize >= minLH) {
    covers = covers - 1
  }

  // Set line height
  let lh = covers * lineHeight / fontSize
  if (lh != lineHeight) {
    res.lineHeight = roundP(lh)
  }

  // Changing the line height moves the block out of the baseline, let's
  // restore it.
  if (fontSize > lineHeight && lineHeight * covers != fontSize) {
    res.position = "relative",
    res.top = `${roundP(0.9 - lineHeight / fontSize)}em`
  }

  return res
}

function flowBlock(lineHeight, fontSize) {
  return {
    ...verticalFlow(lineHeight, fontSize),
    margin: `0 0 ${roundP(lineHeight / fontSize)}em 0`,
  }
}

function paddedBox(lineHeight, factor, border) {
  factor = factor || 0.5
  border = border || 0
  let p = factor * lineHeight
  const res = {
    padding: `${p}em`,
  }
  if (border > 0) {
    res.padding = `calc(${p}em - ${border}px) ${p}em`
  }

  return res
}

function roundP(n) {
  return n.toPrecision(4)
}

function proseCSS(theme) {
  return {
    "h1": {
      fontSize: "2.4em",
      ...flowBlock(1.5, 2.4),
    },
    "h2": {
      fontSize: "2.2em",
      ...flowBlock(1.5, 2.2),
    },
    "h3": {
      fontSize: "1.6em",
      ...flowBlock(1.5, 1.6),
    },
    "h4": {
      fontSize: "1.2em",
      ...flowBlock(1.5, 1.2),
    },
    "h5": {
      fontSize: "1em",
      ...flowBlock(1.5, 1),
      fontWeight: "bold",
    },
    "h6": {
      fontSize: "0.9em",
      ...flowBlock(1.5, 0.9),
      fontWeight: "bold",
    },

    "p, blockquote, address, figure, hr": {
      ...flowBlock(1.5, 1),
      padding: 0,
    },

    "p, li, dd": {
      textAlign: "justify",
    },

    "strong, time, b": {
      fontWeight: "bold",
    },

    "a, a:visited": {
      "color": theme("colors.primary.DEFAULT"),
    },

    "a:focus, a:hover, a:active": {
      "color": theme("colors.primary.dark"),
      "textDecoration": "underline",
    },

    "em, dfn, i": {
      fontStyle: "italic",
    },

    "sub, sup": {
      fontSize: "75%",
      lineHeight: 0,
      position: "relative",
      verticalAlign: "baseline",
    },

    "sup": {
      top: "-.5em",
    },

    "sub": {
      bottom: "-.25em",
    },

    "small": {
      fontSize: "80%",
    },

    "blockquote": {
      background: "rgba(0,0,0,.03)",
      borderLeft: `5px solid ${theme("colors.gray.300")}`,
      ...paddedBox(1.5),

      "*:last-child": {
        marginBottom: 0,
      },
    },

    "cite": {
      fontStyle: "italic",
    },

    "q:before": {
      content: "open-quote",
    },
    "q:after": {
      content: "close-quote",
    },

    "pre": {
      fontSize: "0.9em",
      ...flowBlock(1.5, 0.9),
      ...paddedBox(1.5/0.9, 0.5, 1),
      border: `1px solid ${theme("colors.gray.300")}`,
      backgroundColor: "rgba(0,0,0,.03)",
      whiteSpace: "pre-wrap",

      "code": {
        padding: 0,
        border: 0,
        backgroundColor: "transparent",
        color: "inherit",
      },
    },

    "code, kbd, samp, var": {
      fontSize: "0.875em",
      lineHeight: 1,
      padding: "1px 3px",
      borderRadius: theme("borderRadius.sm"),
      backgroundColor: "rgba(0,0,0,.04)",
    },

    "mark": {
      lineHeight: 1,
      padding: "1px 3px",
      backgroundColor: theme("colors.yellow.300"),
    },

    "img": {
      maxWith: "100%",
    },

    "figure": {
      display: "inline-block",
      width: "auto",
      marginLeft: "auto",
      marginRight: "auto",
      border: `1px solid ${theme("colors.gray.200")}`,
      ...paddedBox(1.5, 0.5, 1),

      "img, svg, pre": {
        display: "block",
        margin: "0 auto",
      },

      "figcaption": {
        fontSize: "0.9em",
        ...verticalFlow(1.5, 0.9),
      },

      "*:last-child": {
        marginBottom: 0,
      },
    },

    "ul, ol, dl": {
      ...flowBlock(1.5, 1),
      padding: 0,
    },

    "ul, ol": {
      paddingLeft: "1.5em",
    },
    "ul": {
      listStyle: "disc",
    },
    "ol": {
      listStyle: "decimal",
    },

    "li": {
      "p, ul, ol": {
        marginTop: 0,
        marginBottom: 0,
      },
    },

    "dl": {
      "dt": {
        fontWeight: "bold",
        margin: 0,
      },
      "dd": {
        margin: 0,
        padding: 0,
        marginLeft: "1.5em",

        "> ul, > ol": {
          paddingLeft: "1em",
        },
      },
    },

    "table": {
      ...flowBlock(1.5, 1),
      tableLayout: "fixed",
      borderCollapse: "collapse",
      borderSpacing: 0,
      marginTop: "-2px",
    },

    "caption": {
      color: theme("colors.gray.800"),
      fontStyle: "italic",
      marginBottom: 0,
    },

    "td, th": {
      ...paddedBox(1.5, 0.25, 0.5),
      verticalAlign: "top",
      minWidth: "2em",
      textAlign: "left",
      border: `1px solid ${theme("colors.gray.400")}`,
    },

    "th": {
      fontWeight: "bold",
      backgroundColor: "rgba(0,0,0,.03)",
    },

    "thead tr:last-child th": {
      borderBottomColor: theme("colors.gray.700"),
    },

    "tfoot": {
      "td, th": {
        fontStyle: "italic",
      },
    },
  }
}

module.exports = plugin(function({addComponents, theme}) {
  addComponents({
    ".prose-grid": {
      backgroundImage: "linear-gradient(#F5D6F5 1px, transparent 1px)",
      backgroundSize: "100% 1.5em",
    },
    ".prose": proseCSS(theme),
  })
})
