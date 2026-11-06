package com.chenbenja.daywire

import android.util.Log

/**
 * Logcat lines for this app. Call sites pass ids and package names only.
 * Notification text, nudge text, and coordinates do not belong here.
 */
internal object Trace {
    private const val TAG = "Daywire"

    fun info(line: String) {
        Log.i(TAG, line)
    }
}
