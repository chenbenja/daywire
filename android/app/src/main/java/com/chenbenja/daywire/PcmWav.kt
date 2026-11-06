package com.chenbenja.daywire

import java.io.Closeable
import java.io.EOFException
import java.io.InputStream

internal enum class SpeechInputKind {
    BAD_WAV,
    READ,
}

internal class SpeechInputException(val kind: SpeechInputKind) : IllegalArgumentException(kind.name)

/**
 * Reads a 16 kHz mono 16-bit PCM WAV. Other encodings are rejected.
 * Samples are scaled by 1/32768, matching the laptop spike.
 */
internal class PcmWav(private val input: InputStream) : Closeable {
    private var dataRemaining = 0L
    private var closed = false

    init {
        val riff = readFully(12)
        if (riff.decodeToString(0, 4) != "RIFF" || riff.decodeToString(8, 12) != "WAVE") {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        var format = -1
        var channels = -1
        var rate = -1
        var bits = -1
        var sawData = false
        while (!sawData) {
            val chunk = try {
                readFully(8)
            } catch (error: SpeechInputException) {
                break
            }
            val id = chunk.decodeToString(0, 4)
            val size = le32(chunk, 4).toLong() and 0xffffffffL
            when (id) {
                "fmt " -> {
                    if (size < 16 || size > 1024) {
                        throw SpeechInputException(SpeechInputKind.BAD_WAV)
                    }
                    val payload = readFully(size.toInt())
                    format = le16(payload, 0)
                    channels = le16(payload, 2)
                    rate = le32(payload, 4)
                    bits = le16(payload, 14)
                    if (size % 2L == 1L) {
                        skipFully(1)
                    }
                }
                "data" -> {
                    dataRemaining = size
                    sawData = true
                }
                else -> {
                    if (size > 16L * 1024 * 1024) {
                        throw SpeechInputException(SpeechInputKind.BAD_WAV)
                    }
                    skipFully(size)
                    if (size % 2L == 1L) {
                        skipFully(1)
                    }
                }
            }
        }
        if (!sawData || format != 1 || channels != 1 || bits != 16 || rate != SPEECH_SAMPLE_RATE) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
    }

    fun exhausted(): Boolean = dataRemaining < 2

    fun readSamples(maxSamples: Int): FloatArray {
        if (closed || maxSamples <= 0 || dataRemaining < 2) {
            return FloatArray(0)
        }
        val samples = minOf(maxSamples.toLong(), dataRemaining / 2).toInt()
        val bytes = readFully(samples * 2)
        dataRemaining -= bytes.size.toLong()
        val out = FloatArray(bytes.size / 2)
        var i = 0
        while (i < out.size) {
            val lo = bytes[i * 2].toInt() and 0xff
            val hi = bytes[i * 2 + 1].toInt() and 0xff
            var v = (hi shl 8) or lo
            if (v and 0x8000 != 0) {
                v -= 0x10000
            }
            out[i] = v / 32768f
            i++
        }
        return out
    }

    override fun close() {
        if (!closed) {
            closed = true
            input.close()
        }
    }

    private fun readFully(n: Int): ByteArray {
        val buf = ByteArray(n)
        var off = 0
        try {
            while (off < n) {
                val read = input.read(buf, off, n - off)
                if (read < 0) {
                    throw EOFException()
                }
                off += read
            }
        } catch (error: SpeechInputException) {
            throw error
        } catch (error: Exception) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        return buf
    }

    private fun skipFully(n: Long) {
        var left = n
        try {
            while (left > 0) {
                val skipped = input.skip(left)
                if (skipped > 0) {
                    left -= skipped
                    continue
                }
                if (input.read() < 0) {
                    throw EOFException()
                }
                left--
            }
        } catch (error: SpeechInputException) {
            throw error
        } catch (error: Exception) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
    }
}

private fun le16(buf: ByteArray, at: Int): Int {
    return (buf[at].toInt() and 0xff) or ((buf[at + 1].toInt() and 0xff) shl 8)
}

private fun le32(buf: ByteArray, at: Int): Int {
    return le16(buf, at) or (le16(buf, at + 2) shl 16)
}
