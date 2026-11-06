package com.chenbenja.daywire

import android.app.Notification
import android.content.pm.PackageManager
import android.os.SystemClock
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification

/**
 * Turns an allow-listed notification into a redacted note.
 *
 * The platform category constant is passed through. The title and the text
 * are not read: this class does not touch the notification extras.
 * Flag checks are local. The allow-list read, the app-label lookup, and the
 * store write run off this callback.
 */
class DaywireNotificationListener : NotificationListenerService() {
    private val dedupe = NotificationDedupe(DEDUPE_WINDOW_MS)

    override fun onNotificationPosted(sbn: StatusBarNotification?) {
        if (sbn == null) {
            return
        }
        val pkg = sbn.packageName ?: return
        if (pkg == packageName) {
            return
        }
        val flags = sbn.notification?.flags ?: 0
        if (shouldSkipPostedFlags(flags)) {
            return
        }
        val key = sbn.key ?: return
        val category = sbn.notification?.category ?: ""
        val posted = sbn.postTime
        DaywireCore.submit(this) { core ->
            if (!AllowList.allows(this, pkg)) {
                return@submit
            }
            if (!dedupe.accept(key, SystemClock.elapsedRealtime())) {
                return@submit
            }
            val label = applicationLabel(packageManager, pkg)
            core.insertNotification(pkg, label, category, posted)
        }
    }

    private fun applicationLabel(manager: PackageManager, pkg: String): String {
        return try {
            val info = manager.getApplicationInfo(pkg, 0)
            manager.getApplicationLabel(info)?.toString().orEmpty().ifBlank { pkg }
        } catch (error: PackageManager.NameNotFoundException) {
            pkg
        }
    }

    private companion object {
        const val DEDUPE_WINDOW_MS = 60_000L
    }
}

/** Ongoing and foreground-service posts are not life events, and neither are group summaries. */
internal fun shouldSkipPostedFlags(flags: Int): Boolean {
    return flags and Notification.FLAG_GROUP_SUMMARY != 0 ||
        flags and Notification.FLAG_ONGOING_EVENT != 0 ||
        flags and Notification.FLAG_FOREGROUND_SERVICE != 0
}

/**
 * Accepts a status-bar key at most once per [windowMs]. Call from one thread.
 * The stored values are keys and timestamps, not notification text.
 */
internal class NotificationDedupe(private val windowMs: Long) {
    private val seen = LinkedHashMap<String, Long>()

    fun accept(key: String, nowMs: Long): Boolean {
        prune(nowMs)
        val previous = seen[key]
        if (previous != null && nowMs - previous < windowMs) {
            return false
        }
        seen[key] = nowMs
        return true
    }

    private fun prune(nowMs: Long) {
        val iterator = seen.entries.iterator()
        while (iterator.hasNext()) {
            val entry = iterator.next()
            if (nowMs - entry.value >= windowMs) {
                iterator.remove()
            }
        }
    }
}
