package com.chenbenja.daywire

import android.content.Context
import android.os.Handler
import android.os.Looper
import androidx.annotation.WorkerThread
import com.chenbenja.daywire.core.mobile.Core
import com.chenbenja.daywire.core.mobile.Mobile
import java.io.File
import java.util.concurrent.Callable
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors

/**
 * One on-device store for the process. Opens and writes run on a single
 * background thread. [call] blocks the caller, so the main thread must use
 * [submit] or [whenReady] instead.
 *
 * The generated types come from `./hack/android-bind.sh` (`-javapkg`).
 */
internal object DaywireCore {
    private val lock = Any()
    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-core").apply { isDaemon = true }
    }
    private val main = Handler(Looper.getMainLooper())

    @Volatile
    private var core: Core? = null

    @Volatile
    var openError: String? = null
        private set

    fun isOpen(): Boolean = core != null

    /** Starts an open if needed. Does not wait. */
    fun warm(context: Context) {
        whenReady(context) {}
    }

    /**
     * Runs [onMain] on the main looper after the store has been opened, or
     * after the open has failed. Does not block the caller.
     */
    fun whenReady(context: Context, onMain: () -> Unit) {
        val app = context.applicationContext
        executor.execute {
            try {
                ensure(app)
            } catch (error: Throwable) {
                if (core == null) {
                    openError = error.message ?: "store open failed"
                }
                Trace.info("store open failed")
            }
            main.post(onMain)
        }
    }

    /** Runs [block] on the store thread and does not wait. */
    fun submit(context: Context, block: (Core) -> Unit) {
        val app = context.applicationContext
        executor.execute {
            try {
                block(ensure(app))
            } catch (error: Throwable) {
                if (core == null) {
                    openError = error.message ?: "store open failed"
                }
                Trace.info("store call failed")
            }
        }
    }

    /**
     * Runs [block] on the store thread and waits. Throws if called on the main
     * thread: the Go call can take as long as its timeout.
     */
    @WorkerThread
    fun <T> call(context: Context, block: (Core) -> T): T {
        if (Looper.myLooper() == Looper.getMainLooper()) {
            throw IllegalStateException("store call on the main thread")
        }
        val app = context.applicationContext
        try {
            return executor.submit(Callable {
                try {
                    block(ensure(app))
                } catch (error: Throwable) {
                    if (core == null) {
                        openError = error.message ?: "store open failed"
                    }
                    throw error
                }
            }).get()
        } catch (error: ExecutionException) {
            throw error.cause ?: error
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
