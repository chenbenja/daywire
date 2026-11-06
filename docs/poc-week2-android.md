# Daywire Week 2 — Android shell

**Status:** thin Kotlin shell over the Week 1 Go store, November 2026.

Week 2 is an app Chen can open in Android Studio. The Week 1 store runs on the phone: summaries and nudge text are AES-256-GCM, and identifiers, types, times, and source ids stay plaintext. The database file is not whole-file encryption. The laptop MCP server is not in the APK, and the phone does not listen on a port. ntfy stays the laptop fallback. It is not used here, and the app does not request the internet permission.

Speech-to-text, speaker identification, and call audio are not this slice. The folder picker is only a capture hook for a later week.

## What shipped

| Piece | Where | Behavior |
|---|---|---|
| On-device store API | `internal/mobile` | Opens the Week 1 SQLite store. Summaries and nudge text are AES-256-GCM. No socket. Tested with `go test`. |
| gomobile surface | `mobile/` | Thin wrapper the Android build binds. It exists so the generated JNI code, which lives outside this module, can import the API. `internal/` packages cannot be imported from there. |
| Bind script | `hack/android-bind.sh` | `gomobile bind` for `android/arm64` and `android/amd64`. Does not pass `-tags speaker`. |
| App | `android/` | One screen: store status, notification access, allow-list, one coarse visit, folder names, a sample nudge, recent events. |
| Notification capture | `DaywireNotificationListener` | Off until the user turns it on. Allow-list empty until they check apps. Allowed notifications become `note` events. Title and text are not read. |
| Coarse visit | `CoarseVisit` | One approximate fix when the user taps the button. Stored as a 0.01 degree grid. No background tracking. |
| Folder hook | `FolderStore` | Storage Access Framework tree. Top-level names only, capped at 200. File bytes are not opened. |
| Nudge | `LocalNudge` | `NotificationCompat` on this phone. The store channel is `local`. Nothing is posted to ntfy. |

`./hack/make.sh all` is unchanged: `gofmt`, `go vet`, `go test -race`, and a `CGO_ENABLED=0` laptop binary. It does not build the APK. `./hack/make.sh android` is the bind script only.

## Redaction

Notification events are type `note`. The source id is `android:notification:<package>`. The summary is:

```text
Notification: <app label> (<package>)
Notification: <app label> (<package>), <category word>
```

The app label is the name from the package manager, capped and stripped of control characters. The category word is included only when Android's `Notification.category` is one of the platform constants (`email`, `msg`, `call`, and the rest of the fixed list in `internal/mobile/redact.go`). Any other category string is dropped, so a body stuffed into that field is not stored. The Go method `InsertNotification` has no title parameter and no body parameter. The listener does not read notification extras.

Group summaries, ongoing notifications (`FLAG_ONGOING_EVENT`), and foreground-service notifications are skipped, and so are notifications posted by Daywire itself (the sample nudge must not become a loop). The same status-bar key is stored at most once per 60 seconds. An incoming-call notification is usually ongoing, so it does not become an event. Rapid updates collapse into one note per 60 seconds when the app reuses the same notification. An app that posts a new key for each message does not collapse those messages.

Visit events are type `visit`, source `android:location`. Latitude and longitude are rounded to two decimal degrees (about 1.1 km) and the summary is `Visit: coarse grid 32.08,34.78`. The raw fix is not stored and is not written to logcat. The grid is the redacted summary, so it does show on the Recent events list.

A button inserts one fixed synthetic note (`android:synthetic`) so the store can be checked without personal data. `InsertRedactedNote` will store the summary it is given, so the shell may call it only with that fixed sentence. The notification listener does not call it.

Notification source ids are `android:notification:<package>` in the clear, so which allowed apps notified, and when, can be read without the key. The package name inside the summary is ciphertext. Hashing the source id, and wrapping `data.key` in the Android Keystore, are not done in this slice.

Local nudges use store channel `local`, detail `shown` or `not shown`. Title and message are encrypted like Week 1. The log line records the id and whether it was shown, not the text.

## What is explicitly not in this slice

- Speech-to-text, including Moonshine, Whisper, Zipformer, or any other on-device model
- Speaker identification, sherpa-onnx, onnxruntime, voiceprints, or an enrollment screen
- Hebrew speech models
- Reading call audio, uploading audio, or an MCP audio field
- Linking the `speaker` build tag or the laptop `daywire-speaker` binary into the APK
- Embedding the Week 1 HTTP / MCP server, or listening on any address
- ntfy, Telegram, FCM, or a Play Store listing
- A `LICENSE` file (the README stays **License: TBD**)

## Call recording, honestly

From Android 10 onward, an ordinary app cannot record both sides of a phone call. The voice-call audio source is restricted, the microphone hears the local side only (and often not even that during a call), and an accessibility service is the wrong tool for this. Play policy forbids using accessibility to record calls. This app does not ship an accessibility service and does not request the microphone.

The supported hook is the Storage Access Framework. If the phone's own dialer or recorder already writes files into a folder, the user can grant Daywire that folder. Week 2 lists the names at the top of the folder. It does not open the files, does not run speech-to-text, and does not build a voiceprint. Where the OEM puts those files varies (`Recordings/Call`, `Sounds/Call`, and other names). Daywire does not scan the disk for them. The user picks the folder.

## Open the project

You need Go 1.26 (the same toolchain as the laptop build; `GOTOOLCHAIN=auto` downloads it) and Android Studio with JDK 17. The Gradle project uses Android Gradle Plugin 9.5.1, Gradle 9.6, `compileSdk` 36, and `minSdk` 29 (Android 10).

1. Install [Android Studio](https://developer.android.com/studio). On first launch, finish the setup wizard so it installs the Android SDK. If it asks for an SDK platform, install **Android API 36**. Build-tools 36 are pulled in by the Gradle build.
2. Note the SDK path. Studio shows it under Settings → Languages & Frameworks → Android SDK. It is often `$HOME/Android/Sdk`.
3. From the repository root:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
./hack/android-bind.sh
```

The script always reinstalls the module's `gomobile` and `gobind` into `output/android-bind-bin` (gitignored), then writes `android/app/libs/daywire.aar` only after checking the archive for speech libraries. That archive is gitignored. Re-run the script when the Go API changes. The bind needs an NDK: in the SDK manager, install **NDK (Side by side)**. Android Gradle Plugin 9.5's default is 28.2.13676358. The bind targets are arm64 (phones) and amd64 (the usual emulator). A 32-bit device needs `android/arm` or `android/386` added to `-target` in `hack/android-bind.sh`. The Gradle wrapper jar in this repo is not checksum-verified in this slice.

`go.mod` records `gobind` as a Go tool (`tool golang.org/x/mobile/cmd/gobind`). That is how Go 1.26's gomobile finds the binder. It is not linked into the laptop `daywire` binary. The bind script also forces that Go 1.26 toolchain to stay local: current gomobile writes a patchless `go 1.26` line, and `GOTOOLCHAIN=auto` would then look for a release named `go1.26`, which does not exist.

4. Android Studio → File → Open → select the `android/` directory (not the repository root). Trust the Gradle project. The first sync downloads Gradle 9.6 and the AndroidX libraries.
5. If Gradle says `Missing app/libs/daywire.aar`, the bind script has not been run, or `ANDROID_HOME` was unset when it ran.
6. Choose a device: a phone with USB debugging, or an emulator whose image is **arm64** or **x86_64**. Run the `app` configuration.

Command-line build, after the bind:

```bash
cd android
./gradlew :app:assembleDebug
```

The debug APK is `android/app/build/outputs/apk/debug/app-debug.apk`.

Checked on this branch: `./hack/make.sh all` passes; `./hack/android-bind.sh` writes an AAR whose native libraries are `arm64-v8a` and `x86_64` and which does not contain sherpa or onnxruntime; `./gradlew :app:testDebugUnitTest :app:assembleDebug` succeeds against SDK platform 36, build-tools 36.0.0, and NDK 28.2.13676358. The resulting APK requests notification-post and approximate-location only. It does not request the internet permission. An emulator was not started, so the buttons were not clicked through on a device.

## On a phone

Install the debug build and open Daywire. The store line should say the store is open once the background open finishes. Events live in the app's private no-backup directory (`no_backup/store/events.db` and `data.key` inside the app sandbox). Backup to the cloud is off. `data.key` is a raw 32-byte key file next to the database, protected by the app sandbox and file-based encryption. It is not wrapped in the Android Keystore. A debug build is debuggable, so `adb shell run-as com.chenbenja.daywire` can read both `no_backup/store/data.key` and `events.db`. The key rules otherwise match Week 1: the file is generated unless `DAYWIRE_DATA_KEY` or `DAYWIRE_DATA_KEY_FILE` is set, which they are not on a phone.

### Notification access

1. Tap **Turn on notification access**.
2. In system settings, enable Daywire. The screen is named "Notification access" or "Device & app notifications", depending on the vendor.
3. On Android 13 and newer, a sideloaded app sometimes shows the toggle greyed out. Open the Daywire app info, use the menu, choose **Allow restricted settings**, go back to notification access, and enable it.
4. Return to Daywire. It should say notification access is on, and **Allowed apps: none.**
5. Tap **Choose allowed apps**. Check only the apps you want. Save. Until you do, posted notifications are ignored.
6. Post a notification from an allowed app. Tap **Refresh**, or leave and reopen Daywire. A `note` appears. It names the app. It does not quote the notification.

### Location

1. Tap **Record a coarse visit**.
2. Allow **approximate** location when Android asks. The app does not request precise location and does not request background location.
3. One fix is stored. The recent-events list shows the coarse grid. Logcat does not get the raw coordinates.
4. If you deny the permission, or location is off, nothing is stored. There is no ongoing tracker.

### Folder

1. Tap **Pick a folder** and choose the directory in the system picker.
2. Daywire lists top-level names in the order of Kotlin `List.sorted()` / `String.compareTo`, which compares UTF-16 code units. For the Basic Multilingual Plane that matches code-point order (uppercase before lowercase). Supplementary characters are not ordered by code point. Directories are shown with a trailing slash. More than 200 names are shortened to the first 200 after that sort. A folder with exactly 200 names is not marked as shortened. Names are shown on screen and are not stored, logged, or opened.
3. **Forget folder** drops the permission. It does not delete the files.

### Nudge

1. Tap **Show a sample nudge**.
2. On Android 13 and newer, allow notifications for Daywire.
3. A notification appears: title "Daywire", text "Synthetic nudge: pack the spare charger." The store records it on channel `local`. The phone does not contact ntfy. With notifications denied, the same text is recorded as not shown.

**Insert a synthetic note** writes a fixed redacted sentence so you can see the list without using a real notification.
