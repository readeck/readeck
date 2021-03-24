const path = require("path")
const zlib = require("zlib")

const CompressionPlugin = require("compression-webpack-plugin")
const MiniCssExtractPlugin = require("mini-css-extract-plugin")
const SpriteLoaderPlugin = require("svg-sprite-loader/plugin")
const AssetMap = require("./utils/asset-map")

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
    filename: "[name].[fullhash:8].js",
    clean: {
      keep: x => ["assets.json", ".keep"].includes(x),
    },
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
              postcssOptions: {
                plugins: () => [
                  require("postcss-preset-env")(),
                  require("cssnano")(),
                ],
              },
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
              postcssOptions: {
                plugins: () => [
                  require("postcss-preset-env")(),
                  require("cssnano")(),
                ],
              },
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
          symbolId: (filename, query) => {
            // The symbolId can be the query part of the import
            return query ? query.substring(1) : path.parse(filename).name
          },
          spriteFilename: "icons.[hash:8].svg",
          outputPath: "img/",
        },
      },
    ],
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: "[name].[fullhash:8].css",
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
    new AssetMap(),
  ],
  optimization: {
    minimize: true,
  },
  devtool: process.env.DEV == "1" ? "source-map" : false,
}
