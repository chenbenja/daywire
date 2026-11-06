package com.chenbenja.daywire

import android.content.Context
import com.chenbenja.daywire.core.mobile.Core
import com.chenbenja.daywire.core.mobile.Mobile
import java.io.File
import java.util.concurrent.Callable
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors

/**
 * One on-device store for the process. Calls are serialized on a single thread.
 * The generated types come from `./hack/android-bind.sh` (`-javapkg`).
 */
internal object DaywireCore {
    private val lock = Any()
    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-core").apply { isDaemon = true }
    }

    @Volatile
    private var core: Core? = null

    @Volatile
    var openError: String? = null
        private set

    fun open(context: Context) {
        try {
            call(context) { }
        } catch (error: Exception) {
            openError = error.message ?: "store open failed"
            throw error
        }
    }

    fun <T> call(context: Context, block: (Core) -> T): T {
        val app = context.applicationContext
        try {
            return executor.submit(Callable { block(ensure(app)) }).get()
        } catch (error: ExecutionException) {
            val cause = error.cause
            if (cause is Exception) {
                throw cause
            }
            throw error
        }
    }

    private fun ensure(context: Context): Core {
        synchronized(lock) {
            core?.let { return it }
            val dir = File(context.noBackupFilesDir, "store")
            if (!dir.isDirectory && !dir.mkdirs()) {
                throw IllegalStateException("create store directory")
            }
            val opened = Mobile.open(dir.absolutePath)
            core = opened
            openError = null
            return opened
        }
    }
}
