package com.chenbenja.daywire

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.documentfile.provider.DocumentFile

/**
 * Names chosen for the folder screen. [capped] is true only when [names] is a
 * shortened list, not when the folder has exactly [cap] entries.
 */
internal data class FolderNames(val names: List<String>, val capped: Boolean)

/** Sorts every name, then keeps the first [cap]. Names are not file contents. */
internal fun capSortedNames(names: List<String>, cap: Int): FolderNames {
    val sorted = names.sorted()
    if (cap >= 0 && sorted.size > cap) {
        return FolderNames(sorted.take(cap), capped = true)
    }
    return FolderNames(sorted, capped = false)
}

/**
 * A folder the user picked with the system document picker.
 * [listNames] returns display names only. It does not open file bytes.
 */
internal object FolderStore {
    private const val PREFS = "daywire"
    private const val KEY = "tree_uri"
    private const val NAME_CAP = 200

    fun replace(context: Context, uri: Uri) {
        val previous = current(context)
        if (previous != null && previous != uri) {
            try {
                context.contentResolver.releasePersistableUriPermission(
                    previous,
                    Intent.FLAG_GRANT_READ_URI_PERMISSION,
                )
            } catch (error: SecurityException) {
                Trace.info("previous folder permission already released")
            }
        }
        prefs(context).edit().putString(KEY, uri.toString()).apply()
    }

    fun current(context: Context): Uri? {
        val raw = prefs(context).getString(KEY, null) ?: return null
        val uri = Uri.parse(raw)
        val kept = context.contentResolver.persistedUriPermissions.any { permission ->
            permission.uri == uri && permission.isReadPermission
        }
        return if (kept) uri else null
    }

    fun forget(context: Context) {
        val uri = current(context)
        if (uri != null) {
            try {
                context.contentResolver.releasePersistableUriPermission(
                    uri,
                    Intent.FLAG_GRANT_READ_URI_PERMISSION,
                )
            } catch (error: SecurityException) {
                Trace.info("folder permission already released")
            }
        }
        prefs(context).edit().remove(KEY).apply()
    }

    fun listNames(context: Context, uri: Uri): FolderNames {
        val root = DocumentFile.fromTreeUri(context, uri) ?: return FolderNames(emptyList(), false)
        val names = ArrayList<String>()
        for (child in root.listFiles()) {
            val name = child.name ?: continue
            names.add(if (child.isDirectory) "$name/" else name)
        }
        return capSortedNames(names, NAME_CAP)
    }

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
}
