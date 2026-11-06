package com.chenbenja.daywire

import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.drawable.Drawable
import android.os.Build
import android.os.Bundle
import android.text.Editable
import android.text.TextWatcher
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.appcompat.app.AppCompatActivity
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.chenbenja.daywire.databinding.ActivityAllowListBinding
import com.chenbenja.daywire.databinding.ItemAppBinding

class AllowListActivity : AppCompatActivity() {
    private lateinit var binding: ActivityAllowListBinding
    private val adapter = AppPickerAdapter()
    private val saveGate = AllowListGate()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityAllowListBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.apps.layoutManager = LinearLayoutManager(this)
        binding.apps.adapter = adapter
        binding.buttonAdd.isEnabled = false
        binding.buttonSave.isEnabled = saveGate.saveEnabled
        binding.filter.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) = Unit
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) = Unit
            override fun afterTextChanged(s: Editable?) {
                adapter.filter(s?.toString().orEmpty())
                updateEmpty()
            }
        })
        binding.buttonAdd.setOnClickListener { addTypedPackage() }
        binding.buttonSave.setOnClickListener {
            AllowList.save(this, adapter.selection())
            finish()
        }
        Thread {
            val apps = loadApps()
            val selected = AllowList.packages(this)
            runOnUiThread {
                if (isDestroyed) {
                    return@runOnUiThread
                }
                adapter.submit(apps, selected)
                saveGate.onLoaded()
                binding.buttonAdd.isEnabled = true
                binding.buttonSave.isEnabled = saveGate.saveEnabled
                binding.textEmpty.text = getString(R.string.allow_none)
                updateEmpty()
            }
        }.start()
    }

    private fun addTypedPackage() {
        val name = binding.packageName.text?.toString()?.trim().orEmpty()
        if (!validPackage(name)) {
            binding.packageName.error = getString(R.string.allow_bad_package)
            return
        }
        binding.packageName.error = null
        adapter.addCustom(
            LaunchableApp(
                packageName = name,
                label = name,
                icon = null,
            ),
        )
        binding.packageName.setText("")
        updateEmpty()
    }

    private fun updateEmpty() {
        binding.textEmpty.visibility = if (adapter.itemCount == 0) View.VISIBLE else View.GONE
    }

    private fun loadApps(): List<LaunchableApp> {
        val intent = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        val resolved = if (Build.VERSION.SDK_INT >= 33) {
            packageManager.queryIntentActivities(intent, PackageManager.ResolveInfoFlags.of(0))
        } else {
            @Suppress("DEPRECATION")
            packageManager.queryIntentActivities(intent, 0)
        }
        val seen = HashSet<String>()
        val apps = ArrayList<LaunchableApp>()
        for (info in resolved) {
            val pkg = info.activityInfo?.packageName ?: continue
            if (pkg == packageName || !seen.add(pkg)) {
                continue
            }
            val label = info.loadLabel(packageManager)?.toString()?.ifBlank { pkg } ?: pkg
            val icon = try {
                info.loadIcon(packageManager)
            } catch (error: Exception) {
                null
            }
            apps.add(LaunchableApp(pkg, label, icon))
        }
        apps.sortBy { it.label.lowercase() }
        return apps
    }

    companion object {
        // Same shape as cleanPackage in internal/mobile/redact.go.
        private val packagePattern = Regex("^[A-Za-z][A-Za-z0-9_]*(\\.[A-Za-z][A-Za-z0-9_]*)+$")

        private fun validPackage(name: String): Boolean {
            return name.length <= 107 && packagePattern.matches(name)
        }
    }
}

internal data class LaunchableApp(
    val packageName: String,
    val label: String,
    val icon: Drawable?,
)

internal class AppPickerAdapter : RecyclerView.Adapter<AppPickerAdapter.Holder>() {
    private val all = ArrayList<LaunchableApp>()
    private val shown = ArrayList<LaunchableApp>()
    private val selected = HashSet<String>()
    private var query = ""

    fun submit(apps: List<LaunchableApp>, initiallySelected: Set<String>) {
        all.clear()
        all.addAll(apps)
        selected.clear()
        selected.addAll(initiallySelected)
        applyFilter()
    }

    fun addCustom(app: LaunchableApp) {
        if (all.none { it.packageName == app.packageName }) {
            all.add(app)
            all.sortBy { it.label.lowercase() }
        }
        selected.add(app.packageName)
        applyFilter()
    }

    fun selection(): Set<String> = selected.toSet()

    fun filter(query: String) {
        this.query = query.trim()
        applyFilter()
    }

    override fun getItemCount(): Int = shown.size

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder {
        val binding = ItemAppBinding.inflate(LayoutInflater.from(parent.context), parent, false)
        return Holder(binding)
    }

    override fun onBindViewHolder(holder: Holder, position: Int) {
        val app = shown[position]
        holder.binding.label.text = app.label
        holder.binding.packageName.text = app.packageName
        holder.binding.icon.setImageDrawable(app.icon)
        holder.binding.check.isChecked = selected.contains(app.packageName)
        holder.binding.root.setOnClickListener {
            val now = !selected.contains(app.packageName)
            if (now) {
                selected.add(app.packageName)
            } else {
                selected.remove(app.packageName)
            }
            holder.binding.check.isChecked = now
        }
    }

    private fun applyFilter() {
        shown.clear()
        val needle = query.lowercase()
        shown.addAll(
            all.filter { app ->
                needle.isEmpty() ||
                    app.label.lowercase().contains(needle) ||
                    app.packageName.lowercase().contains(needle)
            },
        )
        notifyDataSetChanged()
    }

    class Holder(val binding: ItemAppBinding) : RecyclerView.ViewHolder(binding.root)
}
