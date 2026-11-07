package com.chenbenja.daywire

import android.media.AudioFormat
import android.media.MediaCodec
import android.media.MediaExtractor
import android.media.MediaFormat
import java.io.File
import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * Decodes one compressed audio file with the platform decoder, then converts
 * it to 16 kHz mono. Used for a picked recording. It does not open the
 * microphone.
 */
internal class DeviceAudio(file: File) : SpeechSamples {
    private val extractor = MediaExtractor()
    private var codec: MediaCodec? = null
    private var rate: ToSpeechRate? = null
    private var channels = 1
    private var encoding = AudioFormat.ENCODING_PCM_16BIT
    private var inputDone = false
    private var outputDone = false
    private var started = false
    private var closed = false
    private val info = MediaCodec.BufferInfo()

    init {
        try {
            extractor.setDataSource(file.absolutePath)
            var track = -1
            var index = 0
            while (index < extractor.trackCount) {
                val format = extractor.getTrackFormat(index)
                val mime = format.getString(MediaFormat.KEY_MIME).orEmpty()
                if (mime.startsWith("audio/")) {
                    track = index
                    break
                }
                index++
            }
            if (track < 0) {
                throw SpeechInputException(SpeechInputKind.BAD_WAV)
            }
            extractor.selectTrack(track)
            val format = extractor.getTrackFormat(track)
            val mime = format.getString(MediaFormat.KEY_MIME)
                ?: throw SpeechInputException(SpeechInputKind.BAD_WAV)
            val decoder = MediaCodec.createDecoderByType(mime)
            codec = decoder
            decoder.configure(format, null, null, 0)
            decoder.start()
            started = true
            if (format.containsKey(MediaFormat.KEY_SAMPLE_RATE)) {
                adopt(format)
            }
        } catch (error: SpeechInputException) {
            close()
            throw error
        } catch (error: Throwable) {
            close()
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
    }

    override fun exhausted(): Boolean {
        if (!outputDone) {
            return false
        }
        val conv = rate ?: return true
        return !conv.canPull()
    }

    override fun readSamples(maxSamples: Int): FloatArray {
        if (closed || maxSamples <= 0) {
            return FloatArray(0)
        }
        val out = FloatArray(maxSamples)
        var filled = 0
        var stalls = 0
        while (filled < maxSamples) {
            val conv = rate
            if (conv != null) {
                val pulled = conv.pull(maxSamples - filled)
                if (pulled.isNotEmpty()) {
                    pulled.copyInto(out, filled)
                    filled += pulled.size
                    stalls = 0
                    continue
                }
            }
            if (outputDone) {
                break
            }
            if (!pump()) {
                stalls++
                if (stalls > 100) {
                    throw SpeechInputException(SpeechInputKind.BAD_WAV)
                }
            } else {
                stalls = 0
            }
        }
        return if (filled == maxSamples) out else out.copyOf(filled)
    }

    override fun close() {
        if (closed) {
            return
        }
        closed = true
        val decoder = codec
        codec = null
        if (decoder != null) {
            try {
                if (started) {
                    decoder.stop()
                }
            } catch (error: Throwable) {
                // The decoder may already be stopped after a failed start.
            }
            try {
                decoder.release()
            } catch (error: Throwable) {
                // Release is best-effort on the way out.
            }
        }
        try {
            extractor.release()
        } catch (error: Throwable) {
            // The extractor may not have been started.
        }
    }

    private fun adopt(format: MediaFormat) {
        if (!format.containsKey(MediaFormat.KEY_SAMPLE_RATE)) {
            return
        }
        val sampleRate = format.getInteger(MediaFormat.KEY_SAMPLE_RATE)
        val count = if (format.containsKey(MediaFormat.KEY_CHANNEL_COUNT)) {
            format.getInteger(MediaFormat.KEY_CHANNEL_COUNT)
        } else {
            1
        }
        if (sampleRate !in 1..192_000 || count !in 1..8) {
            throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
        channels = count
        encoding = if (format.containsKey(MediaFormat.KEY_PCM_ENCODING)) {
            format.getInteger(MediaFormat.KEY_PCM_ENCODING)
        } else {
            AudioFormat.ENCODING_PCM_16BIT
        }
        if (rate == null) {
            rate = ToSpeechRate(sampleRate)
        }
    }

    private fun pump(): Boolean {
        val decoder = codec ?: return false
        var progress = false
        if (!inputDone) {
            val inIndex = decoder.dequeueInputBuffer(20_000)
            if (inIndex >= 0) {
                val buf = decoder.getInputBuffer(inIndex)
                if (buf == null) {
                    return false
                }
                buf.clear()
                val n = extractor.readSampleData(buf, 0)
                if (n < 0) {
                    decoder.queueInputBuffer(inIndex, 0, 0, 0L, MediaCodec.BUFFER_FLAG_END_OF_STREAM)
                    inputDone = true
                } else {
                    decoder.queueInputBuffer(inIndex, 0, n, extractor.sampleTime, 0)
                    extractor.advance()
                }
                progress = true
            }
        }
        val outIndex = decoder.dequeueOutputBuffer(info, 20_000)
        when {
            outIndex == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED -> {
                adopt(decoder.outputFormat)
                progress = true
            }
            outIndex >= 0 -> {
                try {
                    if (rate == null) {
                        adopt(decoder.outputFormat)
                    }
                    val buf = decoder.getOutputBuffer(outIndex)
                    if (buf != null && info.size > 0) {
                        buf.position(info.offset)
                        buf.limit(info.offset + info.size)
                        buf.order(ByteOrder.LITTLE_ENDIAN)
                        pushPcm(buf)
                    }
                    if (info.flags and MediaCodec.BUFFER_FLAG_END_OF_STREAM != 0) {
                        outputDone = true
                        rate?.finish()
                    }
                    progress = true
                } finally {
                    decoder.releaseOutputBuffer(outIndex, false)
                }
            }
        }
        return progress
    }

    private fun pushPcm(buf: ByteBuffer) {
        val conv = rate ?: return
        when (encoding) {
            AudioFormat.ENCODING_PCM_FLOAT -> {
                val width = channels * 4
                while (buf.remaining() >= width) {
                    var sum = 0.0
                    var channel = 0
                    while (channel < channels) {
                        val sample = buf.float
                        sum += if (sample.isFinite()) sample.coerceIn(-1f, 1f).toDouble() else 0.0
                        channel++
                    }
                    conv.add((sum / channels).toFloat())
                }
            }
            AudioFormat.ENCODING_PCM_8BIT -> {
                while (buf.remaining() >= channels) {
                    var sum = 0.0
                    var channel = 0
                    while (channel < channels) {
                        val unsigned = buf.get().toInt() and 0xff
                        sum += (unsigned - 128) / 128.0
                        channel++
                    }
                    conv.add((sum / channels).toFloat())
                }
            }
            AudioFormat.ENCODING_PCM_16BIT -> {
                val width = channels * 2
                while (buf.remaining() >= width) {
                    var sum = 0.0
                    var channel = 0
                    while (channel < channels) {
                        sum += buf.short / 32768.0
                        channel++
                    }
                    conv.add((sum / channels).toFloat())
                }
            }
            else -> throw SpeechInputException(SpeechInputKind.BAD_WAV)
        }
    }
}
