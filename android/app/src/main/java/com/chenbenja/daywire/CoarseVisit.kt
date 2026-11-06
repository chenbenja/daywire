package com.chenbenja.daywire

import android.annotation.SuppressLint
import android.content.Context
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
 *
 * Status is kept in [VisitSession] so a rotation can still show it. The
 * capture does not hold the Activity.
 */
internal object VisitSession {
    @Volatile
    var inFlight: Boolean = false
        private set

    @Volatile
    private var listener: (() -> Unit)? = null

    @Volatile
    private var pendingStatus: String? = null

    private val main = Handler(Looper.getMainLooper())

    fun setListener(listener: (() -> Unit)?) {
        this.listener = listener
    }

    fun finish(status: String) {
        inFlight = false
        pendingStatus = status
        dispatch()
    }

    fun begin() {
        inFlight = true
        pendingStatus = null
        dispatch()
    }

    fun takeStatus(): String? {
        val status = pendingStatus
        pendingStatus = null
        return status
    }

    private fun dispatch() {
        val current = listener
        main.post { current?.invoke() }
    }
}

internal object CoarseVisit {
    private const val TIMEOUT_MS = 20_000L
    private val callbacks = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-location").apply { isDaemon = true }
    }

    @SuppressLint("MissingPermission")
    fun capture(context: Context) {
        if (VisitSession.inFlight) {
            return
        }
        val app = context.applicationContext
        val missing = app.getString(R.string.location_missing)
        val stored = app.getString(R.string.location_stored)
        val failed = app.getString(R.string.action_failed)
        val denied = app.getString(R.string.location_permission_denied)
        val off = app.getString(R.string.location_off)
        val noProvider = app.getString(R.string.location_no_network_provider)
        val timedOut = app.getString(R.string.location_timeout)
        val manager = app.getSystemService(LocationManager::class.java)
        val locationOn = try {
            manager.isLocationEnabled
        } catch (error: Exception) {
            false
        }
        val network = try {
            manager.isProviderEnabled(LocationManager.NETWORK_PROVIDER)
        } catch (error: IllegalArgumentException) {
            false
        }
        if (!locationOn) {
            VisitSession.finish(off)
            return
        }
        if (!network) {
            VisitSession.finish(noProvider)
            return
        }
        VisitSession.begin()
        val cancel = CancellationSignal()
        val finished = AtomicBoolean(false)
        val main = Handler(Looper.getMainLooper())
        val timeout = Runnable {
            if (finished.compareAndSet(false, true)) {
                cancel.cancel()
                VisitSession.finish(timedOut)
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
                        VisitSession.finish(missing)
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
                    VisitSession.finish(shown)
                },
            )
        } catch (error: SecurityException) {
            if (finished.compareAndSet(false, true)) {
                main.removeCallbacks(timeout)
                VisitSession.finish(denied)
            }
        }
    }
}
