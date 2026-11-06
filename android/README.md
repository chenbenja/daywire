# Daywire Android shell

Open this directory in Android Studio after the Go archive is built. Notification access, location, and the folder picker are walked through in [docs/poc-week2-android.md](../docs/poc-week2-android.md).

Summaries and nudge text are AES-256-GCM. Identifiers, types, times, and source names stay in the clear so they can be filtered. The database file itself is not encrypted. The key is `data.key` next to the database in the app's private no-backup directory, protected by the app sandbox, not by the Android Keystore. Wrapping that key in the Keystore is not done in this slice.

From the repository root:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
./hack/android-bind.sh
```

Then Android Studio → File → Open → this `android/` directory.
