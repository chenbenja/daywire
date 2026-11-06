package com.chenbenja.daywire

import android.app.Notification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ShellGuardsTest {
    @Test
    fun rotationBeforeLoadKeepsTheRestoredSelection() {
        val restored = setOf("com.example.mail")
        assertEquals(
            restored,
            allowSelectionForRotation(loaded = false, current = emptySet(), restored = restored),
        )
        assertEquals(
            emptySet<String>(),
            allowSelectionForRotation(loaded = true, current = emptySet(), restored = restored),
        )
        assertEquals(
            null,
            allowSelectionForRotation(loaded = false, current = emptySet(), restored = null),
        )
    }

    @Test
    fun rotationKeepsAnUncheckedCustomRow() {
        val restored = listOf("com.example.custom")
        assertEquals(
            restored,
            allowCustomForRotation(loaded = false, current = emptyList(), restored = restored),
        )
        assertEquals(
            listOf("com.example.custom"),
            allowCustomForRotation(
                loaded = true,
                current = listOf("com.example.custom"),
                restored = emptyList(),
            ),
        )
    }

    @Test
    fun saveStaysOffUntilTheListHasLoaded() {
        val gate = AllowListGate()
        assertFalse(gate.saveEnabled)
        gate.onLoaded()
        assertTrue(gate.saveEnabled)
    }

    @Test
    fun namesAreSortedBeforeTheCap() {
        val listing = capSortedNames(listOf("c.txt", "a.txt", "b.txt"), 2)
        assertEquals(listOf("a.txt", "b.txt"), listing.names)
        assertTrue(listing.capped)
    }

    @Test
    fun anExactCapIsNotMarkedCapped() {
        val listing = capSortedNames(listOf("b.txt", "a.txt"), 2)
        assertEquals(listOf("a.txt", "b.txt"), listing.names)
        assertFalse(listing.capped)
    }

    @Test
    fun ongoingForegroundAndGroupSummariesAreSkipped() {
        assertTrue(shouldSkipPostedFlags(Notification.FLAG_ONGOING_EVENT))
        assertTrue(shouldSkipPostedFlags(Notification.FLAG_FOREGROUND_SERVICE))
        assertTrue(shouldSkipPostedFlags(Notification.FLAG_GROUP_SUMMARY))
        assertFalse(shouldSkipPostedFlags(0))
    }

    @Test
    fun theSameKeyIsOneEventInsideTheWindow() {
        val dedupe = NotificationDedupe(windowMs = 60_000L)
        assertTrue(dedupe.accept("pkg/1", nowMs = 1_000L))
        assertFalse(dedupe.accept("pkg/1", nowMs = 30_000L))
        assertTrue(dedupe.accept("pkg/1", nowMs = 61_000L))
        assertTrue(dedupe.accept("pkg/2", nowMs = 61_000L))
    }
}
