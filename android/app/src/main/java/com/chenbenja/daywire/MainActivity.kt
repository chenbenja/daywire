package com.chenbenja.daywire

import android.Manifest
import android.content.ComponentName
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
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
            Thread {
                try {
                    LocalNudge.recordUnshown(this)
                } catch (error: Exception) {
                    runOnUiThread { setStatus(error.message ?: getString(R.string.action_failed)) }
                    return@Thread
                }
                runOnUiThread { setStatus(getString(R.string.nudge_permission_denied)) }
            }.start()
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
        contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
        FolderStore.save(this, uri)
        refreshFolder()
        setStatus(getString(R.string.folder_picked))
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
        binding.buttonForget.setOnClickListener {
            FolderStore.forget(this)
            refreshFolder()
            setStatus(getString(R.string.folder_forgotten))
        }
        binding.buttonNudge.setOnClickListener { onNudge() }
        binding.buttonSynthetic.setOnClickListener { insertSynthetic() }
        binding.buttonRefresh.setOnClickListener { refreshEvents() }
    }

    override fun onResume() {
        super.onResume()
        refreshChrome()
        refreshEvents()
    }

    private fun onVisit() {
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
        CoarseVisit.capture(this) { status ->
            setStatus(status)
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
        showSampleNudge()
    }

    private fun showSampleNudge() {
        Thread {
            val status = try {
                LocalNudge.showSample(this)
            } catch (error: Exception) {
                error.message ?: getString(R.string.action_failed)
            }
            runOnUiThread {
                setStatus(status)
                refreshEvents()
            }
        }.start()
    }

    private fun insertSynthetic() {
        Thread {
            val status = try {
                DaywireCore.call(this) { core ->
                    core.insertRedactedNote(
                        "android:synthetic",
                        "Note: synthetic check from the Daywire shell.",
                        System.currentTimeMillis(),
                    )
                }
                getString(R.string.note_stored)
            } catch (error: Exception) {
                error.message ?: getString(R.string.action_failed)
            }
            runOnUiThread {
                setStatus(status)
                refreshEvents()
            }
        }.start()
    }

    private fun refreshChrome() {
        binding.textListener.text = if (listenerEnabled()) {
            getString(R.string.listener_on)
        } else {
            getString(R.string.listener_off)
        }
        val allowed = AllowList.packages(this)
        binding.textAllow.text = if (allowed.isEmpty()) {
            getString(R.string.allow_empty)
        } else {
            getString(R.string.allow_count, allowed.size)
        }
        val openError = (application as DaywireApp).openError
        val ready = openError == null
        binding.textStore.text = if (ready) {
            getString(R.string.store_open)
        } else {
            getString(R.string.store_closed, openError)
        }
        binding.buttonVisit.isEnabled = ready
        binding.buttonNudge.isEnabled = ready
        binding.buttonSynthetic.isEnabled = ready
        binding.buttonRefresh.isEnabled = ready
        refreshFolder()
    }

    private fun listenerEnabled(): Boolean {
        val flat = Settings.Secure.getString(contentResolver, "enabled_notification_listeners")
            ?: return false
        val mine = ComponentName(this, DaywireNotificationListener::class.java)
        return flat.split(":").any { part ->
            ComponentName.unflattenFromString(part) == mine
        }
    }

    private fun refreshFolder() {
        val uri = FolderStore.current(this)
        binding.buttonForget.isEnabled = uri != null
        if (uri == null) {
            binding.textFolder.text = getString(R.string.folder_none)
            binding.textFiles.text = ""
            return
        }
        binding.textFolder.text = getString(R.string.folder_picked)
        Thread {
            val names = try {
                FolderStore.listNames(this, uri)
            } catch (error: Exception) {
                null
            }
            runOnUiThread {
                if (isDestroyed) {
                    return@runOnUiThread
                }
                binding.textFiles.text = when {
                    names == null -> getString(R.string.folder_unavailable)
                    names.isEmpty() -> getString(R.string.folder_empty)
                    names.size >= 200 -> names.joinToString("\n") + "\n" + getString(R.string.folder_capped)
                    else -> names.joinToString("\n")
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
            } catch (error: Exception) {
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
