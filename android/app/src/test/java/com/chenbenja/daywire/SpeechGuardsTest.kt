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
    fun pcmWavRejectsOtherFormats() {
        assertThrows(SpeechInputException::class.java) {
            PcmWav(ByteArrayInputStream(wav(shortArrayOf(0), rate = 8000))).use { }
        }
        assertThrows(SpeechInputException::class.java) {
            PcmWav(ByteArrayInputStream(wav(shortArrayOf(0, 0), channels = 2))).use { }
        }
        assertThrows(SpeechInputException::class.java) {
            PcmWav(ByteArrayInputStream("not a wav".toByteArray())).use { }
        }
    }

}

private fun wav(samples: ShortArray, rate: Int = 16000, channels: Int = 1): ByteArray {
    val dataBytes = samples.size * 2
    val body = ByteBuffer.allocate(44 + dataBytes).order(ByteOrder.LITTLE_ENDIAN)
    body.put("RIFF".toByteArray())
    body.putInt(36 + dataBytes)
    body.put("WAVE".toByteArray())
    body.put("fmt ".toByteArray())
    body.putInt(16)
    body.putShort(1)
    body.putShort(channels.toShort())
    body.putInt(rate)
    body.putInt(rate * channels * 2)
    body.putShort((channels * 2).toShort())
    body.putShort(16)
    body.put("data".toByteArray())
    body.putInt(dataBytes)
    for (sample in samples) {
        body.putShort(sample)
    }
    return body.array()
}
