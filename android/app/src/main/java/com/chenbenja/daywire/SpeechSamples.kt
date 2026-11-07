package com.chenbenja.daywire

import java.io.BufferedInputStream
import java.io.Closeable
import java.io.File
import java.io.FileInputStream

internal enum class SpeechInputKind {
    BAD_WAV,
    READ,
}

internal class SpeechInputException(val kind: SpeechInputKind) : IllegalArgumentException(kind.name)

/** 16 kHz mono float samples from one picked audio file. */
internal interface SpeechSamples : Closeable {
    fun exhausted(): Boolean

    fun readSamples(maxSamples: Int): FloatArray
}

/**
 * Opens a picked file as 16 kHz mono. PCM WAV is decoded here. Anything else
 * is handed to the platform decoder (a phone voice memo, for example).
 * There is no microphone capture.
 */
internal fun openSpeechSamples(file: File): SpeechSamples {
    val input = BufferedInputStream(FileInputStream(file), 64 * 1024)
    try {
        return PcmWav(input)
    } catch (error: SpeechInputException) {
        input.close()
        if (error.kind != SpeechInputKind.BAD_WAV) {
            throw error
        }
    }
    return DeviceAudio(file)
}
