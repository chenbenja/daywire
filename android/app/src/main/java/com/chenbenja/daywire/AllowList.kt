package com.chenbenja.daywire

import android.content.Context

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
