package com.chenbenja.daywire

import kotlin.math.round

/** 16 kHz, matching the laptop spike and the on-device models. */
internal const val SPEECH_SAMPLE_RATE = 16000

/** Pyannote segmentation 3.0 window. Keep in step with internal/speech. */
internal const val DIARIZE_WINDOW_SECONDS = 10.0

/** English speech-to-text pass. */
internal const val ENGLISH_CHUNK_SECONDS = 5.0

/** ivrit-ai Zipformer2 streaming chunk (64 frames). */
internal const val HEBREW_CHUNK_SECONDS = 1.28

/** Silence appended so the streaming decoder emits its last chunk. */
internal const val HEBREW_FLUSH_SECONDS = 1.5

internal const val MIN_EMBED_SECONDS = 0.5
internal const val MIN_CHUNK_SECONDS = 0.1
internal const val MAX_ENROLL_SECONDS = 30.0
internal const val MAX_PROCESS_SECONDS = 600.0

internal data class SampleSpan(val start: Int, val end: Int)

internal fun sampleCount(seconds: Double): Int {
    if (seconds <= 0.0) {
        return 0
    }
    val n = round(seconds * SPEECH_SAMPLE_RATE).toInt()
    return if (n < 1) 1 else n
}

/**
 * Splits `[0, nSamples)` into pieces of [spanSamples].
 * A trailing piece shorter than [minTail] is dropped when an earlier piece
 * exists. A file shorter than [minTail] is still one span.
 */
internal fun planSpans(nSamples: Int, spanSamples: Int, minTail: Int): List<SampleSpan> {
    if (nSamples <= 0 || spanSamples <= 0) {
        return emptyList()
    }
    val tail = if (minTail < 1) 1 else minTail
    val out = ArrayList<SampleSpan>()
    var start = 0
    while (start < nSamples) {
        var end = start + spanSamples
        if (end > nSamples) {
            end = nSamples
        }
        if (end - start < tail && out.isNotEmpty()) {
            break
        }
        out.add(SampleSpan(start, end))
        if (end == nSamples) {
            break
        }
        start += spanSamples
    }
    return out
}

/** Little-endian float32 bits. Gobind accepts this byte slice, not a float array. */
internal fun packEmbedding(samples: FloatArray): ByteArray {
    val out = ByteArray(samples.size * 4)
    var i = 0
    while (i < samples.size) {
        val bits = java.lang.Float.floatToRawIntBits(samples[i])
        out[i * 4] = (bits and 0xff).toByte()
        out[i * 4 + 1] = (bits ushr 8 and 0xff).toByte()
        out[i * 4 + 2] = (bits ushr 16 and 0xff).toByte()
        out[i * 4 + 3] = (bits ushr 24 and 0xff).toByte()
        i++
    }
    return out
}

internal class SpeechGate {
    private val lock = Any()
    private var busyFlag = false

    val busy: Boolean
        get() = synchronized(lock) { busyFlag }

    /** Returns false when work is already running. */
    fun begin(): Boolean {
        synchronized(lock) {
            if (busyFlag) {
                return false
            }
            busyFlag = true
            return true
        }
    }

    fun finish() {
        synchronized(lock) {
            busyFlag = false
        }
    }
}
