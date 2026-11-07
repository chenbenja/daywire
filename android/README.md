# Daywire Android shell

Build a debug APK from the repository root. Android Studio is optional.

```bash
./hack/make.sh apk
```

The APK is `app/build/outputs/apk/debug/app-debug.apk` in this directory. On Linux, with `ANDROID_HOME` unset, that command installs the command-line SDK at `$HOME/Android/Sdk` (platform 36, build-tools 36.0.0, NDK 28.2.13676358), fetches the pinned speech models, binds the Go archive, and assembles the app. It needs Go 1.26, JDK 17 or newer, curl, and unzip.

Install that APK on your own Android 10 or newer arm64 phone. A 32-bit-only phone will not run it. The sideload steps, notification access (including Android 13+ **Allow restricted settings**), location, and the folder picker are in [docs/poc-week2-android.md](../docs/poc-week2-android.md). Speech, including a public sample clip, is in [docs/poc-week3-speech.md](../docs/poc-week3-speech.md).

Summaries and nudge text are AES-256-GCM. Identifiers, types, times, and source names stay in the clear so they can be filtered. The database file itself is not encrypted. The key is `data.key` next to the database in the app's private no-backup directory, protected by the app sandbox, not by the Android Keystore. Wrapping that key in the Keystore is not done in this slice.

The fetch is part of `./hack/make.sh apk`. Without the model files the app still opens and says speech models are not packaged. With them, enroll and process run on device. Speech uses its own file picker. The folder list still does not open files.

To open the project in Android Studio later, bind and fetch first, then File → Open → this `android/` directory:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
./hack/android-bind.sh
./hack/fetch-speech-models.sh
```
