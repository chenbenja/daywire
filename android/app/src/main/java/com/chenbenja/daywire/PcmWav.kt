package com.chenbenja.daywire

import java.io.Closeable
import java.io.EOFException
import java.io.InputStream
import kotlin.math.ceil
import kotlin.math.floor
import kotlin.math.max
import kotlin.math.min

/**
 * PCM WAV converted to 16 kHz mono float. Integer PCM (8, 16, 24, 32-bit),
 * 32-bit float, mono or several channels, and WAVEFORMATEXTENSIBLE are
 * accepted. Other encodings are rejected so the caller can try a compressed
 * decoder. Samples are scaled to about [-1, 1], matching the laptop spike.
 */
internal class PcmWav(private val input: InputStream) : Closeable, SpeechSamples {
    private val channels: Int
    private val bytesPerSample: Int
    private val frameBytes: Int
    private val floatFormat: Boolean
    private var dataRemaining: Long = 0
    private val rate: ToSpeechRate
    private var sourceDone = false
    private var closed = false

    init {
        val riff = readFully(12)
        if (riff.decodeToString(0, 4) != "RIFF" || riff.decodeToString(8, 12) != "WAVE") {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        var format = -1
        var channelCount = -1
        var sampleRate = -1
        var bits = -1
        var blockAlign = -1
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
                    channelCount = le16(payload, 2)
                    sampleRate = le32(payload, 4)
                    blockAlign = le16(payload, 12)
                    bits = le16(payload, 14)
                    if (format == WAVE_EXTENSIBLE) {
                        if (size < 40) {
                            throw SpeechInputException(SpeechInputKind.BAD_WAV)
                        }
                        format = extensibleFormat(payload)
                    }
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
        val width = when (bits) {
            8, 16, 24, 32 -> bits / 8
            else -> 0
        }
        val pcm = format == 1 || format == 3
        if (!sawData || !pcm || channelCount !in 1..8 || sampleRate !in 1..192_000 || width == 0) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        if (format == 3 && bits != 32) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        if (blockAlign != channelCount * width) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        channels = channelCount
        bytesPerSample = width
        frameBytes = channelCount * width
        floatFormat = format == 3
        rate = ToSpeechRate(sampleRate)
    }

    override fun exhausted(): Boolean {
        if (closed) {
            return true
        }
        if (dataRemaining >= frameBytes) {
            return false
        }
        finishSource()
        return !rate.canPull()
    }

    override fun readSamples(maxSamples: Int): FloatArray {
        if (closed || maxSamples <= 0) {
            return FloatArray(0)
        }
        val out = FloatArray(maxSamples)
        var filled = 0
        while (filled < maxSamples) {
            val pulled = rate.pull(maxSamples - filled)
            if (pulled.isNotEmpty()) {
                pulled.copyInto(out, filled)
                filled += pulled.size
                continue
            }
            if (!feedFrame()) {
                val tail = rate.pull(maxSamples - filled)
                if (tail.isEmpty()) {
                    break
                }
                tail.copyInto(out, filled)
                filled += tail.size
            }
        }
        return if (filled == maxSamples) out else out.copyOf(filled)
    }

    override fun close() {
        if (!closed) {
            closed = true
            input.close()
        }
    }

    private fun feedFrame(): Boolean {
        if (sourceDone || dataRemaining < frameBytes) {
            finishSource()
            return false
        }
        val raw = readFully(frameBytes)
        dataRemaining -= frameBytes.toLong()
        var sum = 0.0
        var channel = 0
        while (channel < channels) {
            sum += decodeSample(raw, channel * bytesPerSample).toDouble()
            channel++
        }
        rate.add((sum / channels).toFloat())
        return true
    }

    private fun finishSource() {
        if (!sourceDone) {
            sourceDone = true
            rate.finish()
        }
    }

    private fun decodeSample(raw: ByteArray, at: Int): Float {
        if (floatFormat) {
            val bits = java.lang.Float.intBitsToFloat(le32(raw, at))
            if (!bits.isFinite()) {
                return 0f
            }
            return bits.coerceIn(-1f, 1f)
        }
        return when (bytesPerSample) {
            1 -> {
                val unsigned = raw[at].toInt() and 0xff
                (unsigned - 128) / 128f
            }
            2 -> {
                var v = le16(raw, at)
                if (v and 0x8000 != 0) {
                    v -= 0x10000
                }
                v / 32768f
            }
            3 -> {
                val b0 = raw[at].toInt() and 0xff
                val b1 = raw[at + 1].toInt() and 0xff
                val b2 = raw[at + 2].toInt() and 0xff
                var v = b0 or (b1 shl 8) or (b2 shl 16)
                if (v and 0x800000 != 0) {
                    v -= 0x1000000
                }
                v / 8388608f
            }
            else -> {
                (le32(raw, at).toDouble() / 2147483648.0).toFloat()
            }
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

/**
 * Pull-based 16 kHz view of one mono stream.
 * Downsampling averages the source samples in each output period.
 * A 16 kHz stream is copied. A slower stream holds each sample.
 */
internal class ToSpeechRate(private val inRate: Int) {
    private val step = inRate.toDouble() / SPEECH_SAMPLE_RATE.toDouble()
    private val samples = ArrayDeque<Float>()
    private var base = 0
    private var pos = 0.0
    private var ended = false

    fun add(sample: Float) {
        if (ended) {
            throw IllegalStateException("audio converter is closed")
        }
        samples.addLast(sample)
    }

    fun finish() {
        ended = true
    }

    fun canPull(): Boolean = nextSample() != null

    fun pull(max: Int): FloatArray {
        if (max <= 0) {
            return FloatArray(0)
        }
        val out = FloatArray(max)
        var n = 0
        while (n < max) {
            val sample = nextSample() ?: break
            out[n] = sample
            n++
            pos += step
            discard()
        }
        return if (n == max) out else out.copyOf(n)
    }

    private fun have(): Int = base + samples.size

    private fun nextSample(): Float? {
        val start = pos
        val end = pos + step
        val last = ceil(end - 1e-12).toInt() - 1
        if (last >= have()) {
            if (!ended) {
                return null
            }
            if (floor(start).toInt() >= have()) {
                return null
            }
        }
        var sum = 0.0
        var weight = 0.0
        var index = floor(start).toInt()
        val limit = min(last, have() - 1)
        while (index <= limit) {
            val lo = max(start, index.toDouble())
            val hi = min(end, index + 1.0)
            val span = hi - lo
            if (span > 0.0) {
                sum += at(index) * span
                weight += span
            }
            index++
        }
        if (weight <= 0.0) {
            return null
        }
        return (sum / weight).toFloat()
    }

    private fun at(index: Int): Float = samples.elementAt(index - base)

    private fun discard() {
        val keepFrom = floor(pos).toInt()
        while (base < keepFrom && samples.isNotEmpty()) {
            samples.removeFirst()
            base++
        }
    }
}

private const val WAVE_EXTENSIBLE = 0xFFFE

private fun extensibleFormat(payload: ByteArray): Int {
    val guid = 24
    val code = le32(payload, guid)
    if (code != 1 && code != 3) {
        throw SpeechInputException(SpeechInputKind.BAD_WAV)
    }
    val tail = byteArrayOf(
        0x00, 0x00, 0x10, 0x00,
        0x80.toByte(), 0x00, 0x00, 0xaa.toByte(), 0x00, 0x38, 0x9b.toByte(), 0x71,
    )
    var i = 0
    while (i < tail.size) {
        if (payload[guid + 4 + i] != tail[i]) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        i++
    }
    return code
}

private fun le16(buf: ByteArray, at: Int): Int {
    return (buf[at].toInt() and 0xff) or ((buf[at + 1].toInt() and 0xff) shl 8)
}

private fun le32(buf: ByteArray, at: Int): Int {
    return le16(buf, at) or (le16(buf, at + 2) shl 16)
}
