module.exports = {
  root: true,
  env: {
    es6: true,
    browser: true,
  },
  extends: [
  ],
  parser: "@babel/eslint-parser",
  parserOptions: {
    sourceType: "module",
  },
  plugins: [],
  overrides: [
  ],
  rules: {
    "no-console": process.env.NODE_ENV === "production" ? "warn" : "off",
    "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",

    "quotes": ["error", "double", {"avoidEscape": true}],
    "semi": ["error", "never"],
    "comma-dangle": ["error", {
      "arrays": "always-multiline",
      "objects": "always-multiline",
      "imports": "always-multiline",
      "exports": "always-multiline",
      "functions": "always-multiline",
    }],
    "comma-spacing": ["error", {"before": false, "after": true}],
    "indent": ["error", 2],
  },
}
