# Daywire Android shell

Open this directory in Android Studio after the Go archive is built. Notification access, location, and the folder picker are walked through in [docs/poc-week2-android.md](../docs/poc-week2-android.md).

From the repository root:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
./hack/android-bind.sh
```

Then Android Studio → File → Open → this `android/` directory.
