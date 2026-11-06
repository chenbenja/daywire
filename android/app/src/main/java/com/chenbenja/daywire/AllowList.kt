package com.chenbenja.daywire

import android.content.Context

/**
 * Save stays disabled until the app list has loaded, so an early tap cannot
 * replace a saved allow-list with an empty one.
 */
/**
 * While the list is still loading, keep the selection restored from the
 * previous instance. Null means there is nothing unsaved to keep.
 */
internal fun allowSelectionForRotation(
    loaded: Boolean,
    current: Set<String>,
    restored: Set<String>?,
): Set<String>? = if (loaded) current else restored

/** Custom rows that are not launchable apps, including ones the user unchecked. */
internal fun allowCustomForRotation(
    loaded: Boolean,
    current: List<String>,
    restored: List<String>,
): List<String> = if (loaded) current else restored

internal class AllowListGate {
    var saveEnabled: Boolean = false
        private set

    fun onLoaded() {
        saveEnabled = true
    }
}

/**
 * Package names the user has allowed. Empty until they save a choice.
 * Names are not notification text, so they stay in app preferences.
 */
internal object AllowList {
    private const val PREFS = "daywire"
    private const val KEY = "allow_packages"

    fun packages(context: Context): Set<String> {
        val stored = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getStringSet(KEY, emptySet())
            ?: emptySet()
        return stored.toSet()
    }

    fun allows(context: Context, packageName: String): Boolean {
        return packages(context).contains(packageName)
    }

    fun save(context: Context, packages: Set<String>) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putStringSet(KEY, HashSet(packages))
            .apply()
    }
}
