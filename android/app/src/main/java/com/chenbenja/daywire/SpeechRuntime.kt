package com.chenbenja.daywire

import android.content.Context
import java.io.Closeable
import java.lang.reflect.InvocationTargetException

/** True when the speech runtime archive was on the compile classpath. */
internal fun speechRuntimePackaged(): Boolean {
    return try {
        Class.forName(ENGINE)
        true
    } catch (error: ClassNotFoundException) {
        false
    } catch (error: Throwable) {
        false
    }
}

/**
 * Opens on-device models. [language] is "en", "he", or "" for embedding only.
 * The returned object must be closed. It does not download anything.
 */
internal fun openSpeechModels(context: Context, language: String): SpeechModels {
    try {
        val cls = Class.forName(ENGINE)
        val method = cls.getMethod("open", Context::class.java, String::class.java)
        val opened = method.invoke(null, context.applicationContext, language)
        return opened as SpeechModels
    } catch (error: ClassNotFoundException) {
        throw IllegalStateException("Speech runtime is not packaged.")
    } catch (error: InvocationTargetException) {
        val cause = error.cause
        if (cause is IllegalStateException && safeSpeechMessage(cause.message)) {
            throw cause
        }
        throw IllegalStateException("Speech models could not be loaded.")
    }
}

private fun safeSpeechMessage(message: String?): Boolean {
    return message == "Speech models could not be loaded." ||
        message == "Speech models are not packaged." ||
        message == "Speech runtime is not packaged."
}

private const val ENGINE = "com.chenbenja.daywire.speech.SherpaSpeechEngine"

/**
 * On-device model session. Public so the speech source set can return it
 * from [com.chenbenja.daywire.speech.SherpaSpeechEngine.open]. The shell
 * still opens that class by reflection, and only when the archive is present.
 */
interface SpeechModels : Closeable {
    fun embed(samples: FloatArray): FloatArray

    fun hearWindow(
        samples: FloatArray,
        onTurn: (start: Int, end: Int, embedding: FloatArray?, text: String) -> Unit,
    )
}
