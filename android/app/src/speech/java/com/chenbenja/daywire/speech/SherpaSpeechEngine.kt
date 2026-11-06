package com.chenbenja.daywire.speech

import android.content.Context
import com.chenbenja.daywire.DIARIZE_WINDOW_SECONDS
import com.chenbenja.daywire.ENGLISH_CHUNK_SECONDS
import com.chenbenja.daywire.HEBREW_CHUNK_SECONDS
import com.chenbenja.daywire.HEBREW_FLUSH_SECONDS
import com.chenbenja.daywire.MIN_CHUNK_SECONDS
import com.chenbenja.daywire.MIN_EMBED_SECONDS
import com.chenbenja.daywire.SPEECH_SAMPLE_RATE
import com.chenbenja.daywire.SpeechAssets
import com.chenbenja.daywire.SpeechModels
import com.chenbenja.daywire.planSpans
import com.chenbenja.daywire.sampleCount
import com.k2fsa.sherpa.onnx.FastClusteringConfig
import com.k2fsa.sherpa.onnx.OfflineMoonshineModelConfig
import com.k2fsa.sherpa.onnx.OfflineModelConfig
import com.k2fsa.sherpa.onnx.OfflineRecognizer
import com.k2fsa.sherpa.onnx.OfflineRecognizerConfig
import com.k2fsa.sherpa.onnx.OfflineSpeakerDiarization
import com.k2fsa.sherpa.onnx.OfflineSpeakerDiarizationConfig
import com.k2fsa.sherpa.onnx.OfflineSpeakerSegmentationModelConfig
import com.k2fsa.sherpa.onnx.OfflineSpeakerSegmentationPyannoteModelConfig
import com.k2fsa.sherpa.onnx.OnlineModelConfig
import com.k2fsa.sherpa.onnx.OnlineRecognizer
import com.k2fsa.sherpa.onnx.OnlineRecognizerConfig
import com.k2fsa.sherpa.onnx.OnlineTransducerModelConfig
import com.k2fsa.sherpa.onnx.SpeakerEmbeddingExtractor
import com.k2fsa.sherpa.onnx.SpeakerEmbeddingExtractorConfig
import java.io.File

/**
 * On-device speech runtime. Compiled only when the pinned sherpa-onnx
 * archive is in app/libs. Models are read from app-private files copied
 * out of assets. Nothing is downloaded here.
 */
object SherpaSpeechEngine {
    @JvmStatic
    fun open(context: Context, language: String): SpeechModels {
        val files = SpeechFiles.prepare(context, assetList(language))
        val extractor = SpeakerEmbeddingExtractor(
            config = SpeakerEmbeddingExtractorConfig(
                model = files.getValue(SpeechAssets.EMBEDDING).absolutePath,
                numThreads = 2,
                debug = false,
                provider = "cpu",
            ),
        )
        var diarization: OfflineSpeakerDiarization? = null
        var english: OfflineRecognizer? = null
        var hebrew: OnlineRecognizer? = null
        try {
            if (language == "en" || language == "he") {
                diarization = OfflineSpeakerDiarization(
                    config = OfflineSpeakerDiarizationConfig(
                        segmentation = OfflineSpeakerSegmentationModelConfig(
                            pyannote = OfflineSpeakerSegmentationPyannoteModelConfig(
                                model = files.getValue(SpeechAssets.SEGMENTATION).absolutePath,
                            ),
                            numThreads = 2,
                            debug = false,
                            provider = "cpu",
                        ),
                        embedding = SpeakerEmbeddingExtractorConfig(
                            model = files.getValue(SpeechAssets.EMBEDDING).absolutePath,
                            numThreads = 2,
                            debug = false,
                            provider = "cpu",
                        ),
                        clustering = FastClusteringConfig(numClusters = -1, threshold = 0.5f),
                        minDurationOn = 0.2f,
                        minDurationOff = 0.2f,
                    ),
                )
                if (diarization.sampleRate() != SPEECH_SAMPLE_RATE) {
                    throw IllegalStateException("Speech models could not be loaded.")
                }
                if (language == "he") {
                    hebrew = hebrewRecognizer(files)
                } else {
                    english = englishRecognizer(files)
                }
            }
            return SherpaModels(extractor, diarization, english, hebrew)
        } catch (error: Throwable) {
            hebrew?.release()
            english?.release()
            diarization?.release()
            extractor.release()
            throw IllegalStateException("Speech models could not be loaded.")
        }
    }

    private fun assetList(language: String): List<String> {
        return when (language) {
            "he" -> SpeechAssets.speaker + SpeechAssets.hebrew
            "en" -> SpeechAssets.speaker + SpeechAssets.englishStt
            else -> listOf(SpeechAssets.EMBEDDING)
        }
    }

    private fun englishRecognizer(files: Map<String, File>): OfflineRecognizer {
        return OfflineRecognizer(
            config = OfflineRecognizerConfig(
                modelConfig = OfflineModelConfig(
                    moonshine = OfflineMoonshineModelConfig(
                        preprocessor = files.getValue(SpeechAssets.MOONSHINE_PRE).absolutePath,
                        encoder = files.getValue(SpeechAssets.MOONSHINE_ENC).absolutePath,
                        uncachedDecoder = files.getValue(SpeechAssets.MOONSHINE_UNC).absolutePath,
                        cachedDecoder = files.getValue(SpeechAssets.MOONSHINE_DEC).absolutePath,
                    ),
                    tokens = files.getValue(SpeechAssets.MOONSHINE_TOK).absolutePath,
                    numThreads = 2,
                    debug = false,
                    provider = "cpu",
                ),
            ),
        )
    }

    private fun hebrewRecognizer(files: Map<String, File>): OnlineRecognizer {
        return OnlineRecognizer(
            config = OnlineRecognizerConfig(
                modelConfig = OnlineModelConfig(
                    transducer = OnlineTransducerModelConfig(
                        encoder = files.getValue(SpeechAssets.HEBREW_ENC).absolutePath,
                        decoder = files.getValue(SpeechAssets.HEBREW_DEC).absolutePath,
                        joiner = files.getValue(SpeechAssets.HEBREW_JOIN).absolutePath,
                    ),
                    tokens = files.getValue(SpeechAssets.HEBREW_TOK).absolutePath,
                    numThreads = 2,
                    debug = false,
                    provider = "cpu",
                    modelType = "zipformer2",
                ),
            ),
        )
    }
}

private class SherpaModels(
    private val extractor: SpeakerEmbeddingExtractor,
    private val diarization: OfflineSpeakerDiarization?,
    private val english: OfflineRecognizer?,
    private val hebrew: OnlineRecognizer?,
) : SpeechModels {
    private var closed = false

    override fun embed(samples: FloatArray): FloatArray {
        if (samples.size < sampleCount(MIN_EMBED_SECONDS)) {
            throw IllegalStateException("Speech models could not be loaded.")
        }
        val stream = extractor.createStream()
        try {
            stream.acceptWaveform(samples, SPEECH_SAMPLE_RATE)
            stream.inputFinished()
            if (!extractor.isReady(stream)) {
                throw IllegalStateException("Speech models could not be loaded.")
            }
            val vec = extractor.compute(stream)
            if (vec.isEmpty()) {
                throw IllegalStateException("Speech models could not be loaded.")
            }
            return vec
        } finally {
            stream.release()
        }
    }

    override fun hearWindow(
        samples: FloatArray,
        onTurn: (start: Int, end: Int, embedding: FloatArray?, text: String) -> Unit,
    ) {
        val diar = diarization ?: throw IllegalStateException("Speech models could not be loaded.")
        val ranges = try {
            val segments = diar.process(samples)
            if (segments.isNullOrEmpty()) {
                emptyList()
            } else {
                segments.mapNotNull { segment ->
                    val start = (segment.start * SPEECH_SAMPLE_RATE).toInt().coerceIn(0, samples.size)
                    val end = (segment.end * SPEECH_SAMPLE_RATE).toInt().coerceIn(start, samples.size)
                    if (end - start < sampleCount(0.2)) {
                        null
                    } else {
                        start to end
                    }
                }
            }
        } catch (error: Throwable) {
            emptyList()
        }
        // A full window with no segments is silence. A short file can be one
        // turn when the segmentation model returns nothing.
        val shortClip = samples.size < sampleCount(DIARIZE_WINDOW_SECONDS)
        val turns = when {
            ranges.isNotEmpty() -> ranges
            shortClip && samples.size >= sampleCount(MIN_EMBED_SECONDS) -> listOf(0 to samples.size)
            else -> emptyList()
        }
        val minEmbed = sampleCount(MIN_EMBED_SECONDS)
        for ((start, end) in turns) {
            val slice = samples.copyOfRange(start, end)
            val embedding = if (slice.size >= minEmbed) embed(slice) else null
            val text = transcribe(slice)
            onTurn(start, end, embedding, text)
        }
    }

    override fun close() {
        if (closed) {
            return
        }
        closed = true
        hebrew?.release()
        english?.release()
        diarization?.release()
        extractor.release()
    }

    private fun transcribe(samples: FloatArray): String {
        val moonshine = english
        if (moonshine != null) {
            return transcribeEnglish(moonshine, samples)
        }
        val zipformer = hebrew ?: return ""
        return transcribeHebrew(zipformer, samples)
    }

    private fun transcribeEnglish(recognizer: OfflineRecognizer, samples: FloatArray): String {
        val parts = ArrayList<String>()
        val chunks = planSpans(
            samples.size,
            sampleCount(ENGLISH_CHUNK_SECONDS),
            sampleCount(MIN_CHUNK_SECONDS),
        )
        for (chunk in chunks) {
            val stream = recognizer.createStream()
            try {
                stream.acceptWaveform(samples.copyOfRange(chunk.start, chunk.end), SPEECH_SAMPLE_RATE)
                recognizer.decode(stream)
                val text = recognizer.getResult(stream).text.trim()
                if (text.isNotEmpty()) {
                    parts.add(text)
                }
            } finally {
                stream.release()
            }
        }
        return parts.joinToString(" ")
    }

    private fun transcribeHebrew(recognizer: OnlineRecognizer, samples: FloatArray): String {
        val stream = recognizer.createStream("")
        try {
            val chunks = planSpans(
                samples.size,
                sampleCount(HEBREW_CHUNK_SECONDS),
                sampleCount(MIN_CHUNK_SECONDS),
            )
            for (chunk in chunks) {
                stream.acceptWaveform(samples.copyOfRange(chunk.start, chunk.end), SPEECH_SAMPLE_RATE)
                while (recognizer.isReady(stream)) {
                    recognizer.decode(stream)
                }
            }
            stream.acceptWaveform(FloatArray(sampleCount(HEBREW_FLUSH_SECONDS)), SPEECH_SAMPLE_RATE)
            stream.inputFinished()
            while (recognizer.isReady(stream)) {
                recognizer.decode(stream)
            }
            return recognizer.getResult(stream).text.trim()
        } finally {
            stream.release()
        }
    }
}

private object SpeechFiles {
    fun prepare(context: Context, assets: List<String>): Map<String, File> {
        val root = File(context.noBackupFilesDir, "speech-models")
        if (!root.isDirectory && !root.mkdirs()) {
            throw IllegalStateException("Speech models could not be loaded.")
        }
        val out = LinkedHashMap<String, File>()
        for (asset in assets) {
            val dest = File(root, asset)
            copyAsset(context, asset, dest)
            out[asset] = dest
        }
        return out
    }

    private fun copyAsset(context: Context, asset: String, dest: File) {
        val parent = dest.parentFile
        if (parent != null && !parent.isDirectory && !parent.mkdirs()) {
            throw IllegalStateException("Speech models are not packaged.")
        }
        val expected = try {
            context.assets.openFd(asset).use { it.length }
        } catch (error: Exception) {
            throw IllegalStateException("Speech models are not packaged.")
        }
        if (dest.isFile && dest.length() == expected && expected > 0L) {
            return
        }
        val tmp = File(parent, dest.name + ".partial")
        try {
            context.assets.open(asset).use { input ->
                tmp.outputStream().use { output -> input.copyTo(output) }
            }
            if (tmp.length() != expected || expected <= 0L) {
                throw IllegalStateException("Speech models are not packaged.")
            }
            if (dest.exists() && !dest.delete()) {
                throw IllegalStateException("Speech models are not packaged.")
            }
            if (!tmp.renameTo(dest)) {
                throw IllegalStateException("Speech models are not packaged.")
            }
        } catch (error: IllegalStateException) {
            tmp.delete()
            throw error
        } catch (error: Exception) {
            tmp.delete()
            throw IllegalStateException("Speech models are not packaged.")
        }
    }
}
