package com.chenbenja.daywire

import android.content.Context
import java.io.IOException

/**
 * Asset paths filled by hack/fetch-speech-models.sh. The names match
 * internal/speech/pins.go. Missing files mean that model was not packaged.
 */
internal object SpeechAssets {
    const val EMBEDDING = "speech/embedding.onnx"
    const val SEGMENTATION = "speech/segmentation.onnx"
    const val MOONSHINE_PRE = "speech/moonshine/preprocess.onnx"
    const val MOONSHINE_ENC = "speech/moonshine/encode.int8.onnx"
    const val MOONSHINE_UNC = "speech/moonshine/uncached_decode.int8.onnx"
    const val MOONSHINE_DEC = "speech/moonshine/cached_decode.int8.onnx"
    const val MOONSHINE_TOK = "speech/moonshine/tokens.txt"
    const val HEBREW_ENC = "speech/zipformer-he/encoder.int8.onnx"
    const val HEBREW_DEC = "speech/zipformer-he/decoder.int8.onnx"
    const val HEBREW_JOIN = "speech/zipformer-he/joiner.int8.onnx"
    const val HEBREW_TOK = "speech/zipformer-he/tokens.txt"

    val speaker: List<String> = listOf(EMBEDDING, SEGMENTATION)

    val englishStt: List<String> = listOf(
        MOONSHINE_PRE,
        MOONSHINE_ENC,
        MOONSHINE_UNC,
        MOONSHINE_DEC,
        MOONSHINE_TOK,
    )

    val hebrew: List<String> = listOf(HEBREW_ENC, HEBREW_DEC, HEBREW_JOIN, HEBREW_TOK)

    fun present(context: Context, paths: List<String>): Boolean {
        for (path in paths) {
            if (!has(context, path)) {
                return false
            }
        }
        return true
    }

    fun has(context: Context, path: String): Boolean {
        return try {
            context.assets.open(path).close()
            true
        } catch (error: IOException) {
            false
        }
    }
}
