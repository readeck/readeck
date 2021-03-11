const path = require("path")

const { CleanWebpackPlugin } = require("clean-webpack-plugin")
const HtmlWebpackPlugin = require("html-webpack-plugin")
const MiniCssExtractPlugin = require("mini-css-extract-plugin")
const SpriteLoaderPlugin = require("svg-sprite-loader/plugin")

const mode = process.env.NODE_ENV || "development"
const prod = mode === "production"

const minify = mode === "production"

module.exports = {
  mode,
  entry: {
    bundle: ["./src/main.js"],
  },
  resolve: {
    alias: {
      "~": path.resolve("src"),
      "@box": path.resolve("node_modules/boxicons/svg/"),
    },
    extensions: [".js"],
    mainFields: ["browser", "module", "main"],
  },
  output: {
    path: path.join(__dirname, "../assets/www"),
    publicPath: "assets",
    filename: "[name].[hash:8].js",
  },
  module: {
    rules: [
      {
        test: /\.css$/,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
            options: {
              publicPath: "./",
            },
          },
          "css-loader",
          {
            loader: "postcss-loader",
            options: {
              plugins: (loader) => [
                require("postcss-preset-env")(),
                require("cssnano")(),
              ],
            },
          },
        ],
      },
      {
        test: /\.(scss|sass)$/,
        use: [
          MiniCssExtractPlugin.loader,
          "css-loader",
          {
            loader: "postcss-loader",
            options: {
              plugins: (loader) => [
                require("postcss-preset-env")(),
                require("cssnano")(),
              ],
            },
          },
          "sass-loader",
        ],
      },
      {
        test: /\.(woff2?|eot|ttf|otf)(\?.*)?$/i,
        use: [
          {
            loader: "file-loader",
            options: {
              esModule: false,
              name: "fonts/[name].[hash:8].[ext]",
            },
          },
        ],
      },
      {
        test: /\.svg$/,
        loader: "svg-sprite-loader",
        options: {
          extract: true,
          spriteFilename: "icons.[hash:8].svg",
          outputPath: "img/",
        },
      },
    ],
  },
  plugins: [
    new CleanWebpackPlugin({
      cleanOnceBeforeBuildPatterns: [
        '**\/*',
        '!.keep'
      ]
    }),
    new MiniCssExtractPlugin({
      filename: "[name].[hash:8].css",
    }),
    new HtmlWebpackPlugin({
      template: "src/base.gohtml.tpl",
      filename: path.join(__dirname, "../assets/templates/base.gohtml"),
      inject: false,
      minify: false,
    }),
    new SpriteLoaderPlugin({
      plainSprite: true,
    }),
  ],
  optimization: {
    minimize: minify,
  },
  devtool: prod ? false: "source-map",
}
