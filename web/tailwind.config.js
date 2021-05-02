const colors = require("tailwindcss/colors")

module.exports = {
  purge: [
    "src/**/*.js",
    "../assets/templates/**/*.jet.html",
  ],
  darkMode: false, // or 'media' or 'class'
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      black: colors.black,
      white: colors.white,
      gray: colors.warmGray,
      red: colors.red,
      green: colors.lime,
      blue: colors.lightBlue,
      yellow: colors.amber,
      primary: {
        light: colors.lightBlue[300],
        DEFAULT: colors.lightBlue[600],
        dark: colors.lightBlue[800],
      },
    },
    fontFamily: {
      sans: [
        "public sans", "sans-serif",
        "Apple Color Emoji", "Segoe UI Emoji",
        "Segoe UI Symbol", "Noto Color Emoji",
      ],
      serif: [
        "lora", "serif",
        "Apple Color Emoji", "Segoe UI Emoji",
        "Segoe UI Symbol", "Noto Color Emoji",
      ],
    },
    extend: {
      fontSize: {
        "h1": "2.5rem",
        "h2": "2rem",
        "h3": "1.5rem",
      },
      gridTemplateColumns: {
        "bk-tools": "2fr auto auto",
        "cards": "repeat(auto-fill, minmax(14rem, 1fr))",
      },
      height: {
        "max-content": "max-content",
      },
      padding: {
        "16/9": "56.25%",
      },
      width: {
        "md": "28rem",
      },
    },
  },
  variants: {
    extend: {
      backgroundColor: [
        "data-current",
        "group-hf",
        "group-focus-within",
        "hf",
      ],
      backgroundOpacity: [
        "group-hf",
        "hf",
      ],
      borderColor: [
        "data-current",
        "group-hf",
        "hf",
      ],
      borderOpacity: [
        "group-hf",
        "hf",
      ],
      boxShadow: [
        "group-hf",
        "hf",
      ],
      filter: [
        "group-hf",
        "group-hover",
        "group-focus-within",
        "hf",
      ],
      fontWeight: [
        "data-current",
      ],
      opacity: [
        "group-hf",
        "group-hover",
        "group-focus-within",
        "hf",
      ],
      textColor: [
        "data-current",
        "group-hf",
        "group-focus-within",
        "focus-within",
        "hf",
      ],
      textDecoration: [
        "group-hf",
        "hf",
      ],
      textOpacity: [
        "data-current",
        "group-hf",
        "hf",
      ],
    },
  },
  plugins: [
    require("./ui/plugins/interactions"),
    require("./ui/plugins/forms"),
    require("./ui/plugins/prose"),
  ],
}
