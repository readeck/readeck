const path = require("path")
const zlib = require("zlib")

const { CleanWebpackPlugin } = require("clean-webpack-plugin")
const CompressionPlugin = require("compression-webpack-plugin")
const HtmlWebpackPlugin = require("html-webpack-plugin")
const MiniCssExtractPlugin = require("mini-css-extract-plugin")
const SpriteLoaderPlugin = require("svg-sprite-loader/plugin")

const mode = process.env.NODE_ENV || "development"

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
        test: /\.m?js$/,
        exclude: /(node_modules)/,
        use: ["babel-loader"],
      },
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
        "**\/*",
        "!.keep",
      ],
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
    new CompressionPlugin({
      test: /\.(js|css|svg)?$/i,
      filename: "[path][base].gz",
      algorithm: "gzip",
      compressionOptions: {
        level: 9,
      },
      threshold: 4096,
      minRatio: 0.8,
    }),
    new CompressionPlugin({
      test: /\.(js|css|svg)?$/i,
      filename: "[path][base].br",
      algorithm: "brotliCompress",
      compressionOptions: {
        [zlib.constants.BROTLI_PARAM_QUALITY]: 11,
      },
      threshold: 4096,
      minRatio: 0.8,
    }),
  ],
  optimization: {
    minimize: true,
  },
  devtool: process.env.DEV == "1" ? "source-map" : false,
}
