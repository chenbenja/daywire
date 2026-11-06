package com.chenbenja.daywire

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ShellGuardsTest {
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
}
