package com.chenbenja.daywire

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.activity.result.contract.ActivityResultContracts
import java.io.File
import java.io.FileInputStream
import java.util.concurrent.Executors

internal class OpenSpeechDocument : ActivityResultContracts.OpenDocument() {
    override fun createIntent(context: Context, input: Array<String>): Intent {
        return super.createIntent(context, input)
            .addFlags(Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
    }
}

/**
 * Enroll, forget, and process run off the main thread. Audio is read from
 * one picked document into a private temp file, then deleted. The picker
 * path is not stored or logged.
 */
internal object SpeechWork {
    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-speech").apply { isDaemon = true }
    }

    fun enroll(context: Context, uri: Uri) {
        val app = context.applicationContext
        if (!SpeechSession.begin(app.getString(R.string.speech_enrolling))) {
            return
        }
        executor.execute {
            val status = try {
                enrollFile(app, uri)
            } catch (error: Throwable) {
                Trace.info("speech enroll failed")
                publicStatus(app, error)
            }
            SpeechSession.finish(status)
        }
    }

    fun process(context: Context, uri: Uri) {
        val app = context.applicationContext
        val language = SpeechSession.view().language
        if (!SpeechSession.begin(app.getString(R.string.speech_working))) {
            return
        }
        executor.execute {
            val status = try {
                processFile(app, uri, language)
            } catch (error: Throwable) {
                Trace.info("speech process failed")
                publicStatus(app, error)
            }
            SpeechSession.finish(status)
        }
    }

    fun forget(context: Context) {
        val app = context.applicationContext
        if (!SpeechSession.begin(app.getString(R.string.speech_forgetting))) {
            return
        }
        executor.execute {
            val status = try {
                DaywireCore.call(app) { core -> core.forgetOwnerVoiceprint() }
                Trace.info("speech forget ok")
                app.getString(R.string.speech_forgotten)
            } catch (error: Throwable) {
                Trace.info("speech forget failed")
                publicStatus(app, error)
            }
            SpeechSession.finish(status)
        }
    }

    private fun enrollFile(context: Context, uri: Uri): String {
        Trace.info("speech enroll start")
        if (!speechRuntimePackaged()) {
            return context.getString(R.string.speech_runtime_missing)
        }
        if (!SpeechAssets.present(context, listOf(SpeechAssets.EMBEDDING))) {
            return context.getString(R.string.speech_models_missing)
        }
        val staged = stage(context, uri, enrollBytes())
        try {
            FileInputStream(staged.file).use { input ->
                val wav = PcmWav(input)
                val samples = wav.readSamples(sampleCount(MAX_ENROLL_SECONDS))
                val more = wav.readSamples(1).isNotEmpty() || staged.trimmed
                if (samples.size < sampleCount(MIN_EMBED_SECONDS)) {
                    return context.getString(R.string.speech_short)
                }
                openSpeechModels(context, "").use { models ->
                    val vec = models.embed(samples)
                    DaywireCore.call(context) { core -> core.saveOwnerVoiceprint(packEmbedding(vec)) }
                }
                Trace.info("speech enroll ok")
                return if (more) {
                    context.getString(R.string.speech_enrolled_trimmed)
                } else {
                    context.getString(R.string.speech_enrolled)
                }
            }
        } finally {
            staged.file.delete()
        }
    }

    private fun processFile(context: Context, uri: Uri, language: String): String {
        if (language != "en" && language != "he") {
            return context.getString(R.string.speech_failed)
        }
        Trace.info("speech process start language=$language")
        if (!speechRuntimePackaged()) {
            return context.getString(R.string.speech_runtime_missing)
        }
        if (!SpeechAssets.present(context, SpeechAssets.speaker)) {
            return context.getString(R.string.speech_models_missing)
        }
        if (language == "he") {
            if (!SpeechAssets.present(context, SpeechAssets.hebrew)) {
                return context.getString(R.string.speech_hebrew_missing)
            }
        } else if (!SpeechAssets.present(context, SpeechAssets.englishStt)) {
            return context.getString(R.string.speech_models_missing)
        }
        val enrolled = DaywireCore.call(context) { core -> core.ownerEnrolled() }
        if (!enrolled) {
            return context.getString(R.string.speech_none)
        }
        val staged = stage(context, uri, processBytes())
        try {
            FileInputStream(staged.file).use { input ->
                val wav = PcmWav(input)
                val started = System.currentTimeMillis()
                var origin = 0
                var stored = 0
                var sawTurn = false
                val maxSamples = sampleCount(MAX_PROCESS_SECONDS)
                val windowSamples = sampleCount(DIARIZE_WINDOW_SECONDS)
                val minEmbed = sampleCount(MIN_EMBED_SECONDS)
                try {
                    openSpeechModels(context, language).use { models ->
                        while (origin < maxSamples) {
                            val want = minOf(windowSamples, maxSamples - origin)
                            val window = wav.readSamples(want)
                            if (window.isEmpty()) {
                                break
                            }
                            if (window.size < minEmbed && origin > 0) {
                                break
                            }
                            val base = origin
                            models.hearWindow(window) { start, _, embedding, text ->
                                sawTurn = true
                                if (text.isBlank()) {
                                    return@hearWindow
                                }
                                val label = if (embedding == null) {
                                    "unknown"
                                } else {
                                    DaywireCore.call(context) { core -> core.labelOwner(packEmbedding(embedding)) }
                                }
                                val whenMs = started + ((base + start) * 1000L / SPEECH_SAMPLE_RATE)
                                DaywireCore.call(context) { core ->
                                    core.insertSpeechTurn(language, label, text, whenMs)
                                }
                                stored++
                            }
                            origin += window.size
                            if (window.size < want) {
                                break
                            }
                        }
                    }
                } catch (error: Throwable) {
                    Trace.info("speech process failed")
                    if (stored > 0) {
                        return context.getString(R.string.speech_stored_partial, stored)
                    }
                    throw error
                }
                val trimmed = staged.trimmed || (origin >= maxSamples && !wav.exhausted())
                Trace.info("speech process ok language=$language turns=$stored")
                if (stored == 0) {
                    return if (sawTurn) {
                        context.getString(R.string.speech_no_words)
                    } else {
                        context.getString(R.string.speech_no_speech)
                    }
                }
                return if (trimmed) {
                    context.getString(R.string.speech_stored_trimmed, stored)
                } else {
                    context.getString(R.string.speech_stored, stored)
                }
            }
        } finally {
            staged.file.delete()
        }
    }

    private fun stage(context: Context, uri: Uri, maxBytes: Long): Staged {
        var held = false
        try {
            context.contentResolver.takePersistableUriPermission(
                uri,
                Intent.FLAG_GRANT_READ_URI_PERMISSION,
            )
            held = true
        } catch (error: SecurityException) {
            held = false
        }
        try {
            val dest = File.createTempFile("dw-speech-", ".bin", context.cacheDir)
            try {
                val input = context.contentResolver.openInputStream(uri)
                    ?: throw SpeechInputException(SpeechInputKind.READ)
                var trimmed = false
                input.use { stream ->
                    dest.outputStream().use { output ->
                        val buf = ByteArray(64 * 1024)
                        var written = 0L
                        while (true) {
                            val n = stream.read(buf)
                            if (n < 0) {
                                break
                            }
                            if (written + n > maxBytes) {
                                val keep = (maxBytes - written).toInt()
                                if (keep > 0) {
                                    output.write(buf, 0, keep)
                                }
                                trimmed = true
                                break
                            }
                            output.write(buf, 0, n)
                            written += n
                        }
                    }
                }
                return Staged(dest, trimmed)
            } catch (error: SpeechInputException) {
                dest.delete()
                throw error
            } catch (error: Throwable) {
                dest.delete()
                throw SpeechInputException(SpeechInputKind.READ)
            }
        } finally {
            if (held) {
                try {
                    context.contentResolver.releasePersistableUriPermission(
                        uri,
                        Intent.FLAG_GRANT_READ_URI_PERMISSION,
                    )
                } catch (error: SecurityException) {
                    // The grant is already gone.
                }
            }
        }
    }

    private fun publicStatus(context: Context, error: Throwable): String {
        val input = error as? SpeechInputException
        if (input != null) {
            return when (input.kind) {
                SpeechInputKind.BAD_WAV -> context.getString(R.string.speech_bad_wav)
                SpeechInputKind.READ -> context.getString(R.string.speech_read_failed)
            }
        }
        val message = error.message
        val known = listOf(
            R.string.speech_runtime_missing,
            R.string.speech_models_missing,
            R.string.speech_hebrew_missing,
            R.string.speech_load_failed,
        )
        for (id in known) {
            if (message == context.getString(id)) {
                return message
            }
        }
        if (message == "speaker is not enrolled") {
            return context.getString(R.string.speech_none)
        }
        return context.getString(R.string.speech_failed)
    }

    private fun enrollBytes(): Long {
        return sampleCount(MAX_ENROLL_SECONDS).toLong() * 2L + 65_536L
    }

    private fun processBytes(): Long {
        return sampleCount(MAX_PROCESS_SECONDS).toLong() * 2L + 65_536L
    }

    private class Staged(val file: File, val trimmed: Boolean)
}
