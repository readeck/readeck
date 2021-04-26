const plugin = require("tailwindcss/plugin")
const selectorParser = require("postcss-selector-parser")

module.exports = plugin(function({addVariant, config, e}) {
  const prefixClass = function(className) {
    const prefix = config("prefix")
    const getPrefix = typeof prefix === "function" ? prefix : () => prefix
    return `${getPrefix(`.${className}`)}${className}`
  }

  const groupPseudoClassVariant = function(pseudoClass) {
    return ({ modifySelectors, separator }) => {
      return modifySelectors(({ selector }) => {
        return selectorParser(selectors => {
          selectors.walkClasses(classNode => {
            classNode.value = `group-${pseudoClass}${separator}${classNode.value}`
            classNode.parent.insertBefore(
              classNode,
              selectorParser().astSync(`.${prefixClass("group")}:${pseudoClass} `))
          })
        }).processSync(selector)
      })
    }
  }

  addVariant("data-current", ({ modifySelectors, separator }) => {
    modifySelectors(({ className }) => {
      return `.${e(`data-current${separator}${className}`)}[data-current='true']`
    })
  })

  addVariant("group-focus-within", groupPseudoClassVariant("focus-within"))

  addVariant("hf", ({ modifySelectors, separator }) => {
    modifySelectors(({ selector }) => {
      return selectorParser(selectors => {
        const clonedSelectors = selectors.clone();
        [selectors, clonedSelectors].forEach((sel, i) => {
          sel.walkClasses(classNode => {
            classNode.value = `hf${separator}${classNode.value}`
            classNode.parent.insertAfter(classNode, selectorParser.pseudo({ value: `:${i === 0 ? "hover" : "focus"}` }))
          })
        })
        selectors.append(clonedSelectors)
      }).processSync(selector)
    })
  })

  addVariant("group-hf", ({ modifySelectors, separator }) => {
    modifySelectors(({ selector }) => {
      return selectorParser(selectors => {
        const clonedSelectors = selectors.clone();
        [selectors, clonedSelectors].forEach((sel, i) => {
          sel.walkClasses(classNode => {
            classNode.value = `group-hf${separator}${classNode.value}`
            classNode.parent.insertBefore(classNode, selectorParser().astSync(`.${prefixClass("group")}:${i === 0 ? "hover" : "focus"} `))
          })
        })
        selectors.append(clonedSelectors)
      }).processSync(selector)
    })
  })
})
