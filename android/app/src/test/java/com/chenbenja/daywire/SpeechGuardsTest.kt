package com.chenbenja.daywire

import java.io.ByteArrayInputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class SpeechGuardsTest {
    @Test
    fun windowsMatchTheGoPlanner() {
        val rate = SPEECH_SAMPLE_RATE
        assertEquals(
            listOf(SampleSpan(0, 10 * rate), SampleSpan(10 * rate, 20 * rate), SampleSpan(20 * rate, 25 * rate)),
            planSpans(25 * rate, 10 * rate, rate / 2),
        )
        assertEquals(
            listOf(SampleSpan(0, 10 * rate)),
            planSpans(10 * rate + rate / 5, 10 * rate, rate / 2),
        )
        assertEquals(
            listOf(SampleSpan(0, rate / 5)),
            planSpans(rate / 5, 10 * rate, rate / 2),
        )
        assertEquals(emptyList<SampleSpan>(), planSpans(0, 10 * rate, rate / 2))
    }

    @Test
    fun chunkSizesMatchTheModels() {
        assertEquals(10 * SPEECH_SAMPLE_RATE, sampleCount(DIARIZE_WINDOW_SECONDS))
        assertEquals(5 * SPEECH_SAMPLE_RATE, sampleCount(ENGLISH_CHUNK_SECONDS))
        assertEquals(20480, sampleCount(HEBREW_CHUNK_SECONDS))
        assertEquals(24000, sampleCount(HEBREW_FLUSH_SECONDS))
        assertEquals(8000, sampleCount(MIN_EMBED_SECONDS))
    }

    @Test
    fun aSecondSpeechJobDoesNotStart() {
        val gate = SpeechGate()
        assertFalse(gate.busy)
        assertTrue(gate.begin())
        assertTrue(gate.busy)
        assertFalse(gate.begin())
        gate.finish()
        assertFalse(gate.busy)
        assertTrue(gate.begin())
    }

    @Test
    fun pcmWavReadsMono16k() {
        val pcm = wav(shortArrayOf(0, 16384, -16384))
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            val samples = reader.readSamples(8)
            assertEquals(3, samples.size)
            assertEquals(0f, samples[0], 1e-6f)
            assertEquals(0.5f, samples[1], 1e-4f)
            assertEquals(-0.5f, samples[2], 1e-4f)
            assertTrue(reader.exhausted())
            assertEquals(0, reader.readSamples(4).size)
        }
    }

    @Test
    fun pcmWavKeepsTheMostNegativeSample() {
        PcmWav(ByteArrayInputStream(wav(shortArrayOf(Short.MIN_VALUE)))).use { reader ->
            val samples = reader.readSamples(1)
            assertEquals(1, samples.size)
            assertEquals(-1f, samples[0], 1e-6f)
        }
    }

    @Test
    fun packEmbeddingIsLittleEndianFloat32() {
        val packed = packEmbedding(floatArrayOf(1f, -0.5f))
        assertEquals(8, packed.size)
        assertEquals(0x00.toByte(), packed[0])
        assertEquals(0x00.toByte(), packed[1])
        assertEquals(0x80.toByte(), packed[2])
        assertEquals(0x3f.toByte(), packed[3])
        assertEquals(0x00.toByte(), packed[4])
        assertEquals(0x00.toByte(), packed[5])
        assertEquals(0x00.toByte(), packed[6])
        assertEquals(0xbf.toByte(), packed[7])
    }

    @Test
    fun pcmWavDownmixesStereoAndReportsLeftover() {
        val pcm = wav(shortArrayOf(16384, -16384, 3277, 3277), channels = 2)
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            assertFalse(reader.exhausted())
            val first = reader.readSamples(1)
            assertEquals(1, first.size)
            assertEquals(0f, first[0], 1e-4f)
            assertFalse(reader.exhausted())
            val rest = reader.readSamples(4)
            assertEquals(1, rest.size)
            assertEquals(0.1f, rest[0], 1e-3f)
            assertTrue(reader.exhausted())
            assertEquals(0, reader.readSamples(2).size)
        }
    }

    @Test
    fun pcmWavAverages48kDownTo16k() {
        val pcm = wav(shortArrayOf(16384, 0, 0, -16384, -16384, -16384), rate = 48000)
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            val samples = reader.readSamples(8)
            assertEquals(2, samples.size)
            assertEquals(16384f / 3f / 32768f, samples[0], 1e-4f)
            assertEquals(-0.5f, samples[1], 1e-4f)
            assertTrue(reader.exhausted())
        }
    }

    @Test
    fun pcmWavHolds8kSamples() {
        val pcm = wav(shortArrayOf(16384, 0), rate = 8000)
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            val samples = reader.readSamples(8)
            assertEquals(4, samples.size)
            assertEquals(0.5f, samples[0], 1e-4f)
            assertEquals(0.5f, samples[1], 1e-4f)
            assertEquals(0f, samples[2], 1e-4f)
            assertEquals(0f, samples[3], 1e-4f)
            assertTrue(reader.exhausted())
        }
    }

    @Test
    fun pcmWavReads8BitAndFloat() {
        PcmWav(ByteArrayInputStream(wavBytes(byteArrayOf(128.toByte(), 255.toByte()), bits = 8))).use { reader ->
            val samples = reader.readSamples(4)
            assertEquals(2, samples.size)
            assertEquals(0f, samples[0], 1e-6f)
            assertEquals(127f / 128f, samples[1], 1e-4f)
        }
        val one = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putFloat(-0.25f).array()
        PcmWav(ByteArrayInputStream(wavBytes(one, bits = 32, format = 3))).use { reader ->
            val samples = reader.readSamples(2)
            assertEquals(1, samples.size)
            assertEquals(-0.25f, samples[0], 1e-6f)
        }
    }

    @Test
    fun pcmWavReadsExtensiblePcm() {
        val payload = ByteBuffer.allocate(2).order(ByteOrder.LITTLE_ENDIAN).putShort(16384).array()
        PcmWav(ByteArrayInputStream(extensibleWav(payload))).use { reader ->
            val samples = reader.readSamples(2)
            assertEquals(1, samples.size)
            assertEquals(0.5f, samples[0], 1e-4f)
        }
    }

    @Test
    fun aSecondOfAudioDoesNotResampleOncePerFrame() {
        val n = SPEECH_SAMPLE_RATE
        val pcm = wav(ShortArray(n) { 16384 })
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            val samples = reader.readSamples(n)
            assertEquals(n, samples.size)
            assertEquals(0.5f, samples[0], 1e-4f)
            assertEquals(0.5f, samples[n - 1], 1e-4f)
            assertTrue(reader.exhausted())
            // One fill per source frame allocated a FloatArray the size of
            // everything still requested. A 1 s clip is 16000 frames.
            assertTrue(reader.resamplerFills < n / 32)
        }
    }

    @Test
    fun aSecondAt48kDoesNotResampleOncePerFrame() {
        val n = 48_000
        val pcm = wav(ShortArray(n) { 16384 }, rate = 48000)
        PcmWav(ByteArrayInputStream(pcm)).use { reader ->
            val samples = reader.readSamples(SPEECH_SAMPLE_RATE)
            assertEquals(SPEECH_SAMPLE_RATE, samples.size)
            assertEquals(0.5f, samples[0], 1e-4f)
            assertEquals(0.5f, samples[samples.size - 1], 1e-4f)
            assertTrue(reader.exhausted())
            assertTrue(reader.resamplerFills < n / 32)
        }
    }

    @Test
    fun pcmWavRejectsNonPcm() {
        assertThrows(SpeechInputException::class.java) {
            PcmWav(ByteArrayInputStream(wav(shortArrayOf(0), format = 7))).use { }
        }
        assertThrows(SpeechInputException::class.java) {
            PcmWav(ByteArrayInputStream("not a wav".toByteArray())).use { }
        }
    }

}

private fun wav(
    samples: ShortArray,
    rate: Int = 16000,
    channels: Int = 1,
    format: Int = 1,
): ByteArray {
    val payload = ByteBuffer.allocate(samples.size * 2).order(ByteOrder.LITTLE_ENDIAN)
    for (sample in samples) {
        payload.putShort(sample)
    }
    return wavBytes(payload.array(), rate = rate, channels = channels, bits = 16, format = format)
}

private fun wavBytes(
    payload: ByteArray,
    rate: Int = 16000,
    channels: Int = 1,
    bits: Int = 16,
    format: Int = 1,
): ByteArray {
    val width = bits / 8
    val block = channels * width
    val body = ByteBuffer.allocate(44 + payload.size).order(ByteOrder.LITTLE_ENDIAN)
    body.put("RIFF".toByteArray())
    body.putInt(36 + payload.size)
    body.put("WAVE".toByteArray())
    body.put("fmt ".toByteArray())
    body.putInt(16)
    body.putShort(format.toShort())
    body.putShort(channels.toShort())
    body.putInt(rate)
    body.putInt(rate * block)
    body.putShort(block.toShort())
    body.putShort(bits.toShort())
    body.put("data".toByteArray())
    body.putInt(payload.size)
    body.put(payload)
    return body.array()
}

private fun extensibleWav(payload: ByteArray): ByteArray {
    val fmtSize = 40
    val body = ByteBuffer.allocate(12 + 8 + fmtSize + 8 + payload.size).order(ByteOrder.LITTLE_ENDIAN)
    body.put("RIFF".toByteArray())
    body.putInt(body.capacity() - 8)
    body.put("WAVE".toByteArray())
    body.put("fmt ".toByteArray())
    body.putInt(fmtSize)
    body.putShort(0xfffe.toShort())
    body.putShort(1)
    body.putInt(16000)
    body.putInt(32000)
    body.putShort(2)
    body.putShort(16)
    body.putShort(22)
    body.putShort(16)
    body.putInt(0)
    body.putInt(1)
    body.putShort(0)
    body.putShort(0x0010)
    body.put(byteArrayOf(0x80.toByte(), 0x00, 0x00, 0xaa.toByte(), 0x00, 0x38, 0x9b.toByte(), 0x71))
    body.put("data".toByteArray())
    body.putInt(payload.size)
    body.put(payload)
    return body.array()
}
