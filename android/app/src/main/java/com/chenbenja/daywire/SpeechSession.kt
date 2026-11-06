package com.chenbenja.daywire

import android.os.Handler
import android.os.Looper

internal data class SpeechView(
    val language: String,
    val busy: Boolean,
    val status: String?,
    val runtime: Boolean?,
    val english: Boolean?,
    val hebrew: Boolean?,
    val embedding: Boolean?,
    val segmentation: Boolean?,
    val enrolled: Boolean?,
)

/**
 * Speech UI state that outlives the Activity. Inference updates it from a
 * background thread. The listener runs on the main looper.
 */
internal object SpeechSession {
    private val lock = Any()
    private val main = Handler(Looper.getMainLooper())
    private val gate = SpeechGate()
    private var listener: ((Boolean) -> Unit)? = null
    private var languageCode = "en"
    private var statusText: String? = null
    private var runtimeReady: Boolean? = null
    private var englishReady: Boolean? = null
    private var hebrewReady: Boolean? = null
    private var embeddingReady: Boolean? = null
    private var segmentationReady: Boolean? = null
    private var enrolledFlag: Boolean? = null

    fun setListener(listener: ((Boolean) -> Unit)?) {
        synchronized(lock) {
            this.listener = listener
        }
    }

    fun view(): SpeechView {
        synchronized(lock) {
            return SpeechView(
                language = languageCode,
                busy = gate.busy,
                status = statusText,
                runtime = runtimeReady,
                english = englishReady,
                hebrew = hebrewReady,
                embedding = embeddingReady,
                segmentation = segmentationReady,
                enrolled = enrolledFlag,
            )
        }
    }

    fun setLanguage(value: String) {
        val next = if (value == "he") "he" else "en"
        synchronized(lock) {
            if (languageCode == next) {
                return
            }
            languageCode = next
        }
        dispatch(false)
    }

    fun setFacts(
        runtime: Boolean,
        english: Boolean,
        hebrew: Boolean,
        embedding: Boolean,
        segmentation: Boolean,
        enrolled: Boolean?,
    ) {
        synchronized(lock) {
            runtimeReady = runtime
            englishReady = english
            hebrewReady = hebrew
            embeddingReady = embedding
            segmentationReady = segmentation
            enrolledFlag = enrolled
        }
        dispatch(false)
    }

    fun begin(waiting: String): Boolean {
        synchronized(lock) {
            if (!gate.begin()) {
                return false
            }
            statusText = waiting
        }
        dispatch(false)
        return true
    }

    fun finish(status: String) {
        synchronized(lock) {
            gate.finish()
            statusText = status
        }
        dispatch(true)
    }

    private fun dispatch(refreshEvents: Boolean) {
        val current = synchronized(lock) { listener }
        main.post { current?.invoke(refreshEvents) }
    }
}
