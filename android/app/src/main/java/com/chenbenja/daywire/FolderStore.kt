package com.chenbenja.daywire

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.documentfile.provider.DocumentFile

/**
 * A folder the user picked with the system document picker.
 * [listNames] returns display names only. It does not open file bytes.
 */
internal object FolderStore {
    private const val PREFS = "daywire"
    private const val KEY = "tree_uri"
    private const val NAME_CAP = 200

    fun save(context: Context, uri: Uri) {
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

    fun listNames(context: Context, uri: Uri): List<String> {
        val root = DocumentFile.fromTreeUri(context, uri) ?: return emptyList()
        val names = ArrayList<String>()
        for (child in root.listFiles()) {
            val name = child.name ?: continue
            names.add(if (child.isDirectory) "$name/" else name)
            if (names.size >= NAME_CAP) {
                break
            }
        }
        names.sort()
        return names
    }

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
}
