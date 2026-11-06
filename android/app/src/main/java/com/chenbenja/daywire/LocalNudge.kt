package com.chenbenja.daywire

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

/**
 * The phone nudge path. It uses NotificationCompat and records channel "local".
 * It does not call ntfy.
 */
internal object LocalNudge {
    const val CHANNEL_ID = "daywire_nudges"
    private const val SAMPLE_ID = 1001
    private const val SAMPLE_TITLE = "Daywire"
    private const val SAMPLE_TEXT = "Synthetic nudge: pack the spare charger."

    fun ensureChannel(context: Context) {
        if (Build.VERSION.SDK_INT < 26) {
            return
        }
        val manager = context.getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Daywire nudges",
            NotificationManager.IMPORTANCE_DEFAULT,
        )
        channel.description = "Nudges shown on this phone."
        manager.createNotificationChannel(channel)
    }

    fun showSample(context: Context): String {
        val enabled = NotificationManagerCompat.from(context).areNotificationsEnabled()
        if (!enabled) {
            DaywireCore.call(context) { core ->
                core.recordLocalNudge(SAMPLE_TITLE, SAMPLE_TEXT, false)
            }
            return context.getString(R.string.nudge_not_shown)
        }
        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_daywire)
            .setContentTitle(SAMPLE_TITLE)
            .setContentText(SAMPLE_TEXT)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .build()
        NotificationManagerCompat.from(context).notify(SAMPLE_ID, notification)
        DaywireCore.call(context) { core ->
            core.recordLocalNudge(SAMPLE_TITLE, SAMPLE_TEXT, true)
        }
        return context.getString(R.string.nudge_shown)
    }

    fun recordUnshown(context: Context) {
        DaywireCore.call(context) { core ->
            core.recordLocalNudge(SAMPLE_TITLE, SAMPLE_TEXT, false)
        }
    }
}
