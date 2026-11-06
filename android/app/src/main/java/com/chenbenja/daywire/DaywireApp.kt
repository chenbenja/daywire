package com.chenbenja.daywire

import android.app.Application

class DaywireApp : Application() {
    val openError: String?
        get() = DaywireCore.openError

    override fun onCreate() {
        super.onCreate()
        LocalNudge.ensureChannel(this)
        DaywireCore.warm(this)
    }
}
