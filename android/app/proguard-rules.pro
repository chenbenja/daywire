# gomobile JNI. Release minification is off in this slice; these rules are here
# so a later minify pass does not strip the bind.
-keep class go.** { *; }
-keep class com.chenbenja.daywire.core.** { *; }
