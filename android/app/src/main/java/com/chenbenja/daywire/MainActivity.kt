package com.chenbenja.daywire

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.chenbenja.daywire.databinding.ActivityMainBinding
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException
import org.json.JSONArray

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private val whenFormat = DateTimeFormatter.ofPattern("d MMM HH:mm")

    private val notificationPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) {
            showSampleNudge()
        } else {
            LocalNudge.recordUnshown(this)
            setStatus(getString(R.string.nudge_permission_denied))
        }
    }

    private val locationPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) {
            captureVisit()
        } else {
            setStatus(getString(R.string.location_permission_denied))
        }
    }

    private val openTree = registerForActivityResult(
        ActivityResultContracts.OpenDocumentTree(),
    ) { uri ->
        if (uri == null) {
            return@registerForActivityResult
        }
        try {
            contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
        } catch (error: SecurityException) {
            setStatus(getString(R.string.folder_not_persistent))
            return@registerForActivityResult
        }
        binding.buttonForget.isEnabled = false
        val picked = uri
        refreshFolder(FolderChange.PICK) { app ->
            FolderStore.replace(app, picked)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.buttonListener.setOnClickListener {
            startActivity(Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS))
        }
        binding.buttonAllow.setOnClickListener {
            startActivity(Intent(this, AllowListActivity::class.java))
        }
        binding.buttonVisit.setOnClickListener { onVisit() }
        binding.buttonFolder.setOnClickListener { openTree.launch(null) }
        binding.buttonForget.setOnClickListener { forgetFolder() }
        binding.buttonNudge.setOnClickListener { onNudge() }
        binding.buttonSynthetic.setOnClickListener { insertSynthetic() }
        binding.buttonRefresh.setOnClickListener { refreshEvents() }
    }

    override fun onStart() {
        super.onStart()
        VisitSession.setListener {
            if (!isDestroyed) {
                applyVisitSession()
            }
        }
        ActionStatus.setListener {
            if (!isDestroyed) {
                applyActionStatus()
            }
        }
        applyVisitSession()
        applyActionStatus()
    }

    override fun onStop() {
        VisitSession.setListener(null)
        ActionStatus.setListener(null)
        super.onStop()
    }

    override fun onResume() {
        super.onResume()
        refreshChrome()
        DaywireCore.whenReady(this) {
            if (isDestroyed) {
                return@whenReady
            }
            refreshChrome()
            refreshEvents()
        }
    }

    private fun onVisit() {
        if (VisitSession.inFlight) {
            return
        }
        val granted = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_COARSE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED
        if (granted) {
            captureVisit()
        } else {
            locationPermission.launch(Manifest.permission.ACCESS_COARSE_LOCATION)
        }
    }

    private fun captureVisit() {
        if (VisitSession.inFlight) {
            return
        }
        CoarseVisit.capture(this)
    }

    private fun applyVisitSession() {
        binding.buttonVisit.isEnabled = DaywireCore.isOpen() && !VisitSession.inFlight
        if (VisitSession.inFlight) {
            setStatus(getString(R.string.location_waiting))
            return
        }
        val status = VisitSession.takeStatus() ?: return
        setStatus(status)
        refreshEvents()
    }

    private fun applyActionStatus() {
        val ready = DaywireCore.isOpen()
        val status = ActionStatus.snapshot()
        val busy = status?.busy == true
        binding.buttonNudge.isEnabled = ready && !busy
        binding.buttonSynthetic.isEnabled = ready && !busy
        if (status == null) {
            return
        }
        setStatus(status.text)
        if (!status.busy) {
            refreshEvents()
        }
    }

    private fun onNudge() {
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.POST_NOTIFICATIONS,
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
            return
        }
        if (ActionStatus.busy) {
            return
        }
        showSampleNudge()
    }

    private fun showSampleNudge() {
        val app = applicationContext
        if (!ActionStatus.begin(getString(R.string.nudge_working))) {
            return
        }
        Thread {
            val status = try {
                LocalNudge.showSample(app)
            } catch (error: Throwable) {
                error.message ?: app.getString(R.string.action_failed)
            }
            ActionStatus.finish(status)
        }.start()
    }

    private fun insertSynthetic() {
        val app = applicationContext
        if (!ActionStatus.begin(getString(R.string.note_working))) {
            return
        }
        Thread {
            val status = try {
                DaywireCore.call(app) { core ->
                    core.insertRedactedNote(
                        "android:synthetic",
                        "Note: synthetic check from the Daywire shell.",
                        System.currentTimeMillis(),
                    )
                }
                app.getString(R.string.note_stored)
            } catch (error: Throwable) {
                error.message ?: app.getString(R.string.action_failed)
            }
            ActionStatus.finish(status)
        }.start()
    }

    private fun refreshChrome() {
        refreshListener()
        refreshAllowCount()
        val openError = (application as DaywireApp).openError
        val ready = DaywireCore.isOpen()
        binding.textStore.text = when {
            openError != null -> getString(R.string.store_closed, openError)
            ready -> getString(R.string.store_open)
            else -> getString(R.string.store_opening)
        }
        binding.buttonVisit.isEnabled = ready && !VisitSession.inFlight
        binding.buttonNudge.isEnabled = ready && !ActionStatus.busy
        binding.buttonSynthetic.isEnabled = ready && !ActionStatus.busy
        binding.buttonRefresh.isEnabled = ready
        refreshFolder()
    }

    private var listenerGeneration = 0

    private fun refreshListener() {
        val generation = ++listenerGeneration
        val app = applicationContext
        Thread {
            val enabled = try {
                NotificationManagerCompat.getEnabledListenerPackages(app).contains(packageName)
            } catch (error: Throwable) {
                null
            }
            runOnUiThread {
                if (isDestroyed || generation != listenerGeneration || enabled == null) {
                    return@runOnUiThread
                }
                binding.textListener.text = if (enabled) {
                    getString(R.string.listener_on)
                } else {
                    getString(R.string.listener_off)
                }
            }
        }.start()
    }

    private var allowGeneration = 0

    private fun refreshAllowCount() {
        val generation = ++allowGeneration
        Thread {
            val count = try {
                AllowList.packages(applicationContext).size
            } catch (error: Throwable) {
                -1
            }
            runOnUiThread {
                if (isDestroyed || generation != allowGeneration) {
                    return@runOnUiThread
                }
                binding.textAllow.text = when {
                    count < 0 -> getString(R.string.allow_unread)
                    count == 0 -> getString(R.string.allow_empty)
                    else -> getString(R.string.allow_count, count)
                }
            }
        }.start()
    }

    private var folderGeneration = 0

    private fun forgetFolder() {
        binding.buttonForget.isEnabled = false
        refreshFolder(FolderChange.FORGET) { app ->
            FolderStore.forget(app)
        }
    }

    private fun refreshFolder(
        change: FolderChange = FolderChange.NONE,
        mutate: ((android.content.Context) -> Unit)? = null,
    ) {
        val generation = ++folderGeneration
        val app = applicationContext
        Thread {
            var writeFailed = false
            if (mutate != null) {
                try {
                    mutate(app)
                } catch (error: Throwable) {
                    writeFailed = true
                }
            }
            var readFailed = false
            val uri = try {
                FolderStore.current(app)
            } catch (error: Throwable) {
                readFailed = true
                null
            }
            val listing = if (uri == null) {
                null
            } else {
                try {
                    FolderStore.listNames(app, uri)
                } catch (error: Throwable) {
                    null
                }
            }
            runOnUiThread {
                if (isDestroyed || generation != folderGeneration) {
                    return@runOnUiThread
                }
                binding.buttonForget.isEnabled = uri != null
                when {
                    readFailed -> {
                        binding.textFolder.text = getString(R.string.folder_read_failed)
                        binding.textFiles.text = ""
                    }
                    uri == null -> {
                        binding.textFolder.text = getString(R.string.folder_none)
                        binding.textFiles.text = ""
                    }
                    else -> {
                        binding.textFolder.text = getString(R.string.folder_picked)
                        binding.textFiles.text = when {
                            listing == null -> getString(R.string.folder_unavailable)
                            listing.names.isEmpty() -> getString(R.string.folder_empty)
                            listing.capped -> {
                                listing.names.joinToString("\n") + "\n" + getString(R.string.folder_capped)
                            }
                            else -> listing.names.joinToString("\n")
                        }
                    }
                }
                if (!writeFailed && change == FolderChange.FORGET && uri == null) {
                    setStatus(getString(R.string.folder_forgotten))
                } else if (!writeFailed && change == FolderChange.PICK && uri != null) {
                    setStatus(getString(R.string.folder_picked))
                } else if (writeFailed) {
                    setStatus(getString(R.string.folder_write_failed))
                }
            }
        }.start()
    }

    private fun refreshEvents() {
        if ((application as DaywireApp).openError != null) {
            binding.textEvents.text = getString(R.string.events_unavailable)
            return
        }
        Thread {
            val json = try {
                DaywireCore.call(this) { core -> core.listEventsJSON("", 50L) }
            } catch (error: Throwable) {
                null
            }
            val text = if (json == null) {
                getString(R.string.events_unavailable)
            } else {
                formatEvents(json)
            }
            runOnUiThread {
                if (!isDestroyed) {
                    binding.textEvents.text = text
                }
            }
        }.start()
    }

    private fun formatEvents(json: String): String {
        return try {
            val array = JSONArray(json)
            if (array.length() == 0) {
                getString(R.string.events_empty)
            } else {
                buildString {
                    for (i in 0 until array.length()) {
                        if (i > 0) {
                            append("\n\n")
                        }
                        val obj = array.getJSONObject(i)
                        append(obj.getString("type"))
                        append(" · ")
                        append(formatWhen(obj.optString("occurred_at")))
                        append('\n')
                        append(obj.optString("summary"))
                    }
                }
            }
        } catch (error: Exception) {
            getString(R.string.events_unavailable)
        }
    }

    private fun formatWhen(raw: String): String {
        return try {
            Instant.parse(raw).atZone(ZoneId.systemDefault()).format(whenFormat)
        } catch (error: DateTimeParseException) {
            raw
        }
    }

    private fun setStatus(text: String) {
        binding.textStatus.text = text
    }
}

/**
 * In-flight sample-nudge and synthetic-note text. Same shape as [VisitSession]:
 * the Activity can be recreated while the work is still running.
 */
internal object ActionStatus {
    private val lock = Any()
    private val main = Handler(Looper.getMainLooper())
    private var listener: (() -> Unit)? = null
    private var pending: String? = null
    private var busyFlag = false

    val busy: Boolean
        get() = synchronized(lock) { busyFlag }

    fun setListener(listener: (() -> Unit)?) {
        synchronized(lock) {
            this.listener = listener
        }
    }

    /** Returns false when another sample nudge or synthetic note is already running. */
    fun begin(waiting: String): Boolean {
        synchronized(lock) {
            if (busyFlag) {
                return false
            }
            busyFlag = true
            pending = waiting
        }
        dispatch()
        return true
    }

    fun finish(status: String) {
        synchronized(lock) {
            busyFlag = false
            pending = status
        }
        dispatch()
    }

    fun snapshot(): ActionSnapshot? {
        val current = synchronized(lock) {
            val text = pending ?: return null
            val stillBusy = busyFlag
            if (!stillBusy) {
                pending = null
            }
            ActionSnapshot(text, stillBusy)
        }
        return current
    }

    private fun dispatch() {
        val current = synchronized(lock) { listener }
        main.post { current?.invoke() }
    }
}

private enum class FolderChange {
    NONE,
    PICK,
    FORGET,
}

internal data class ActionSnapshot(val text: String, val busy: Boolean)
