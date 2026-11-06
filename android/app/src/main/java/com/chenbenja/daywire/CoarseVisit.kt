package com.chenbenja.daywire

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationManager
import android.os.Build
import android.os.CancellationSignal
import android.os.Handler
import android.os.Looper
import java.util.concurrent.Executors

/**
 * One approximate fix. The coordinates are handed to the Go core, which
 * stores a coarse grid. This file does not log them.
 */
internal object CoarseVisit {
    private val callbacks = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "daywire-location").apply { isDaemon = true }
    }

    @SuppressLint("MissingPermission")
    fun capture(context: Context, onStatus: (String) -> Unit) {
        val manager = context.getSystemService(LocationManager::class.java)
        val provider = when {
            manager.isProviderEnabled(LocationManager.NETWORK_PROVIDER) -> LocationManager.NETWORK_PROVIDER
            manager.isProviderEnabled(LocationManager.PASSIVE_PROVIDER) -> LocationManager.PASSIVE_PROVIDER
            else -> {
                onStatus(context.getString(R.string.location_off))
                return
            }
        }
        val main = Handler(Looper.getMainLooper())
        val deliver: (Location?) -> Unit = { location ->
            if (location == null) {
                main.post { onStatus(context.getString(R.string.location_missing)) }
            } else {
                try {
                    DaywireCore.call(context) { core ->
                        core.insertCoarseVisit(location.latitude, location.longitude, location.time)
                    }
                    main.post { onStatus(context.getString(R.string.location_stored)) }
                } catch (error: Exception) {
                    val text = error.message ?: context.getString(R.string.action_failed)
                    main.post { onStatus(text) }
                }
            }
        }
        try {
            if (Build.VERSION.SDK_INT >= 30) {
                val cancel = CancellationSignal()
                main.postDelayed({ cancel.cancel() }, 20_000)
                manager.getCurrentLocation(provider, cancel, callbacks, deliver)
            } else {
                @Suppress("DEPRECATION")
                manager.requestSingleUpdate(provider, { location -> deliver(location) }, Looper.getMainLooper())
            }
        } catch (error: SecurityException) {
            onStatus(context.getString(R.string.location_permission_denied))
        }
    }
}
