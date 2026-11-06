package com.chenbenja.daywire

import android.app.Application

class DaywireApp : Application() {
    val openError: String?
        get() = DaywireCore.openError

    override fun onCreate() {
        super.onCreate()
        LocalNudge.ensureChannel(this)
        try {
            DaywireCore.open(this)
        } catch (error: Exception) {
            Trace.info("store open failed")
        }
    }
}
