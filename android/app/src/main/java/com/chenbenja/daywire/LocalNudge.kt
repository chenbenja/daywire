package com.chenbenja.daywire

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat

/**
 * The phone nudge path. It uses NotificationCompat and records channel "local".
 * It does not call ntfy. A blocked channel or channel group is recorded as not
 * shown. The store write does not block the caller.
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
        val compat = NotificationManagerCompat.from(context)
        val channel = compat.getNotificationChannel(CHANNEL_ID)
        if (!canShow(context, compat, channel)) {
            record(context, shown = false)
            return context.getString(R.string.nudge_not_shown)
        }
        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_daywire)
            .setContentTitle(SAMPLE_TITLE)
            .setContentText(SAMPLE_TEXT)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .build()
        if (Build.VERSION.SDK_INT < 33 ||
            ContextCompat.checkSelfPermission(
                context,
                Manifest.permission.POST_NOTIFICATIONS,
            ) == PackageManager.PERMISSION_GRANTED
        ) {
            compat.notify(SAMPLE_ID, notification)
            record(context, shown = true)
            return context.getString(R.string.nudge_shown)
        }
        record(context, shown = false)
        return context.getString(R.string.nudge_not_shown)
    }

    fun recordUnshown(context: Context) {
        record(context, shown = false)
    }

    private fun record(context: Context, shown: Boolean) {
        DaywireCore.submit(context) { core ->
            core.recordLocalNudge(SAMPLE_TITLE, SAMPLE_TEXT, shown)
        }
    }

    private fun canShow(
        context: Context,
        compat: NotificationManagerCompat,
        channel: NotificationChannel?,
    ): Boolean {
        val posted = Build.VERSION.SDK_INT < 33 ||
            ContextCompat.checkSelfPermission(
                context,
                Manifest.permission.POST_NOTIFICATIONS,
            ) == PackageManager.PERMISSION_GRANTED
        val channelOpen = channel == null || channel.importance != NotificationManager.IMPORTANCE_NONE
        return posted && compat.areNotificationsEnabled() && channelOpen && !groupBlocked(compat, channel)
    }

    private fun groupBlocked(compat: NotificationManagerCompat, channel: NotificationChannel?): Boolean {
        val groupId = channel?.group ?: return false
        val group = compat.getNotificationChannelGroup(groupId) ?: return false
        return group.isBlocked
    }
}
