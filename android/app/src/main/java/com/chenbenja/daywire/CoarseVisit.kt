package com.chenbenja.daywire

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationManager
import android.os.CancellationSignal
import android.os.Handler
import android.os.Looper
import androidx.core.location.LocationManagerCompat
import androidx.core.util.Consumer
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * One approximate fix from the network provider. The coordinates are handed
 * to the Go core, which stores a coarse grid. This file does not log them.
 *
 * [LocationManagerCompat.getCurrentLocation] supplies the API 29 callback
 * itself, so this file does not declare one.
 */
internal object CoarseVisit {
    private const val TIMEOUT_MS = 20_000L
    private val callbacks = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-location").apply { isDaemon = true }
    }

    @SuppressLint("MissingPermission")
    fun capture(context: Context, alive: () -> Boolean, onStatus: (String) -> Unit) {
        val app = context.applicationContext
        val missing = app.getString(R.string.location_missing)
        val stored = app.getString(R.string.location_stored)
        val failed = app.getString(R.string.action_failed)
        val denied = app.getString(R.string.location_permission_denied)
        val off = app.getString(R.string.location_off)
        val timedOut = app.getString(R.string.location_timeout)
        val main = Handler(Looper.getMainLooper())
        fun postStatus(text: String) {
            main.post {
                if (!alive()) {
                    return@post
                }
                onStatus(text)
            }
        }
        val manager = app.getSystemService(LocationManager::class.java)
        val enabled = try {
            manager.isProviderEnabled(LocationManager.NETWORK_PROVIDER)
        } catch (error: IllegalArgumentException) {
            false
        }
        if (!enabled) {
            postStatus(off)
            return
        }
        val cancel = CancellationSignal()
        val finished = AtomicBoolean(false)
        val timeout = Runnable {
            if (finished.compareAndSet(false, true)) {
                cancel.cancel()
                postStatus(timedOut)
            }
        }
        main.postDelayed(timeout, TIMEOUT_MS)
        try {
            LocationManagerCompat.getCurrentLocation(
                manager,
                LocationManager.NETWORK_PROVIDER,
                cancel,
                callbacks,
                Consumer { location ->
                    if (!finished.compareAndSet(false, true)) {
                        return@Consumer
                    }
                    main.removeCallbacks(timeout)
                    if (location == null) {
                        postStatus(missing)
                        return@Consumer
                    }
                    val shown = try {
                        DaywireCore.call(app) { core ->
                            core.insertCoarseVisit(location.latitude, location.longitude, location.time)
                        }
                        stored
                    } catch (error: Exception) {
                        error.message ?: failed
                    }
                    postStatus(shown)
                },
            )
        } catch (error: SecurityException) {
            if (finished.compareAndSet(false, true)) {
                main.removeCallbacks(timeout)
                postStatus(denied)
            }
        }
    }
}
