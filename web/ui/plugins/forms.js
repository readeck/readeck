const svgToDataUri = require("mini-svg-data-uri")
const plugin = require("tailwindcss/plugin")
const { colors, spacing, borderWidth, borderRadius, outline } = require("tailwindcss/defaultTheme")

module.exports = plugin(function({addComponents, theme}) {
  const rules = {
    [[
      ".form-input",
      ".form-textarea",
      ".form-select",
      ".form-multiselect",
    ]]: {
      appearance: "none",
      backgroundColor: "#fff",
      borderColor: theme("colors.gray.300", colors.gray[300]),
      borderWidth: borderWidth["DEFAULT"],
      borderRadius: borderRadius.DEFAULT,
      padding: spacing[2],
      fontSize: theme("fontSize.base"),
      lineHeight: theme("lineHeight.tight"),

      "&:focus": {
        outline: outline.none[0],
        outlineOffset: outline.none[1],
        "--tw-ring-inset": "var(--tw-empty,/*!*/ /*!*/)",
        "--tw-ring-offset-width": "0px",
        "--tw-ring-offset-color": "#fff",
        "--tw-ring-color": theme("colors.primary.DEFAULT", colors.blue[600]),
        "--tw-ring-offset-shadow": "var(--tw-ring-inset) 0 0 0 var(--tw-ring-offset-width) var(--tw-ring-offset-color)",
        "--tw-ring-shadow": "var(--tw-ring-inset) 0 0 0 calc(1px + var(--tw-ring-offset-width)) var(--tw-ring-color)",
        boxShadow: "var(--tw-ring-offset-shadow), var(--tw-ring-shadow), var(--tw-shadow, 0 0 #0000)",
        borderColor: theme("colors.primary.DEFAULT", colors.blue[600]),
      },
    },

    [[".form-input::placeholder", ".form-textarea::placeholder"]]: {
      color: theme("colors.gray.500", colors.gray[500]),
      opacity: 1,
    },

    ".form-input::-webkit-datetime-edit-fields-wrapper": {
      padding: 0,
    },

    ".form-select": {
      backgroundImage: `url("${svgToDataUri(
        `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 20 20"><path stroke="${theme(
          "colors.gray.500",
          colors.gray[500],
        )}" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M6 8l4 4 4-4"/></svg>`,
      )}")`,
      backgroundPosition: `right ${spacing[2]} center`,
      backgroundRepeat: "no-repeat",
      backgroundSize: "1.5em 1.5em",
      paddingRight: spacing[10],
      colorAdjust: "exact",

      "&[multiple]": {
        backgroundImage: "initial",
        backgroundPosition: "initial",
        backgroundRepeat: "unset",
        backgroundSize: "initial",
        paddingRight: spacing[3],
        colorAdjust: "unset",
      },
    },

    [[".form-checkbox", ".form-radio"]]: {
      appearance: "none",
      padding: "0",
      colorAdjust: "exact",
      display: "inline-block",
      verticalAlign: "baseline",
      backgroundOrigin: "border-box",
      userSelect: "none",
      flexShrink: "0",
      height: spacing[4],
      width: spacing[4],
      color: theme("colors.primary.DEFAULT", colors.blue[600]),
      backgroundColor: "#fff",
      borderColor: theme("colors.gray.500", colors.gray[500]),
      borderWidth: borderWidth["DEFAULT"],

      "&:checked": {
        borderColor: "transparent",
        backgroundColor: "currentColor",
        backgroundSize: "100% 100%",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      },

      "&:focus": {
        outline: outline.none[0],
        outlineOffset: outline.none[1],
        "--tw-ring-inset": "var(--tw-empty,/*!*/ /*!*/)",
        "--tw-ring-offset-width": "2px",
        "--tw-ring-offset-color": "#fff",
        "--tw-ring-color": theme("colors.primary.DEFAULT", colors.blue[600]),
        "--tw-ring-offset-shadow": "var(--tw-ring-inset) 0 0 0 var(--tw-ring-offset-width) var(--tw-ring-offset-color)",
        "--tw-ring-shadow": "var(--tw-ring-inset) 0 0 0 calc(2px + var(--tw-ring-offset-width)) var(--tw-ring-color)",
        boxShadow: "var(--tw-ring-offset-shadow), var(--tw-ring-shadow), var(--tw-shadow, 0 0 #0000)",
      },

      "&:checked:hover, &:checked:focus": {
        borderColor: "transparent",
        backgroundColor: "currentColor",
      },
    },

    ".form-checkbox": {
      borderRadius: 0,

      "&:checked": {
        backgroundImage: `url("${svgToDataUri(
          '<svg viewBox="0 0 16 16" fill="white" xmlns="http://www.w3.org/2000/svg"><path d="M12.207 4.793a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0l-2-2a1 1 0 011.414-1.414L6.5 9.086l4.293-4.293a1 1 0 011.414 0z"/></svg>',
        )}")`,
      },

      "&:indeterminate": {
        backgroundImage: `url("${svgToDataUri(
          '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 16 16"><path stroke="white" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h8"/></svg>',
        )}")`,
        borderColor: "transparent",
        backgroundColor: "currentColor",
        backgroundSize: "100% 100%",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      },

      "&:indeterminate:hover, &:indeterminate:focus": {
        borderColor: "transparent",
        backgroundColor: "currentColor",
      },
    },

    ".form-radio": {
      borderRadius: "100%",

      "&:checked": {
        backgroundImage: `url("${svgToDataUri(
          '<svg viewBox="0 0 16 16" fill="white" xmlns="http://www.w3.org/2000/svg"><circle cx="8" cy="8" r="3"/></svg>',
        )}")`,
      },
    },

    ".form-file": {
      background: "unset",
      borderColor: "inherit",
      borderWidth: 0,
      borderRadius: 0,
      padding: 0,
      fontSize: "unset",
      lineHeight: "inherit",

      "&:focus": {
        outline: [
          "1px solid ButtonText",
          "1px auto -webkit-focus-ring-color",
        ],
      },
    },
  }

  addComponents(rules)
})
