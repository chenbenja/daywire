package com.chenbenja.daywire

import android.app.Notification
import android.content.pm.PackageManager
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification

/**
 * Turns an allow-listed notification into a redacted note.
 *
 * The platform category constant is passed through. The title and the text
 * are not read: this class does not touch the notification extras.
 */
class DaywireNotificationListener : NotificationListenerService() {
    override fun onNotificationPosted(sbn: StatusBarNotification?) {
        if (sbn == null) {
            return
        }
        val pkg = sbn.packageName ?: return
        if (pkg == packageName) {
            return
        }
        val flags = sbn.notification?.flags ?: 0
        if (flags and Notification.FLAG_GROUP_SUMMARY != 0) {
            return
        }
        if (!AllowList.allows(this, pkg)) {
            return
        }
        val label = applicationLabel(packageManager, pkg)
        val category = sbn.notification?.category ?: ""
        val posted = sbn.postTime
        try {
            val id = DaywireCore.call(this) { core ->
                core.insertNotification(pkg, label, category, posted)
            }
            Trace.info("notification stored id=$id package=$pkg")
        } catch (error: Exception) {
            Trace.info("notification not stored package=$pkg")
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
}
