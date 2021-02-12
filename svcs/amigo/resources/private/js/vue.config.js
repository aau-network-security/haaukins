const TerserPlugin = require('terser-webpack-plugin')

module.exports =  {
    chainWebpack: config => {
        if(config.plugins.has('extract-css')) {
            const extractCSSPlugin = config.plugin('extract-css')
            extractCSSPlugin && extractCSSPlugin.tap(() => [{
                filename: '[name].css',
                chunkFilename: '[name].css'
            }])
        }
    },
    configureWebpack: config => {
        config.optimization = {
            splitChunks: {
                chunks: 'all',
                minSize: 10000,
                maxSize: 250000,
            },
            minimize: true,
            minimizer: [
                new TerserPlugin({
                    terserOptions: {
                        ecma: undefined,
                        warnings: false,
                        parse: {},
                        compress: { drop_console: true },
                        mangle: true, // Note `mangle.properties` is `false` by default.
                        module: false,
                        output: { comments: false },
                        toplevel: false,
                        nameCache: null,
                        ie8: false,
                        keep_classnames: undefined,
                        keep_fnames: false,
                        safari10: false,
                    },
                }),
            ],

        }
        config.devtool = false
        config.mode = process.env.NODE_ENV
    }
}