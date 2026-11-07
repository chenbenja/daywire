plugins {
    id("com.android.application")
}

val sherpaAar = file("libs/sherpa-onnx-1.12.28.aar")

android {
    namespace = "com.chenbenja.daywire"
    compileSdk = 36
    ndkVersion = "28.2.13676358"

    defaultConfig {
        applicationId = "com.chenbenja.daywire"
        minSdk = 29
        targetSdk = 36
        versionCode = 1
        versionName = "0.3.0"
        ndk {
            // Match hack/android-bind.sh. arm64-v8a is a phone, x86_64 is the emulator.
            abiFilters += listOf("arm64-v8a", "x86_64")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        viewBinding = true
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }

    androidResources {
        noCompress += listOf("onnx", "txt")
    }

    sourceSets.getByName("main") {
        assets.directories.add("speech-assets")
        if (sherpaAar.isFile) {
            // AGP 9 compiles Kotlin only from the kotlin source set. These
            // files are .kt even though the directory is named java.
            java.directories.add("src/speech/java")
            kotlin.directories.add("src/speech/java")
        }
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.17.0")
    implementation("androidx.appcompat:appcompat:1.7.1")
    implementation("com.google.android.material:material:1.13.0")
    implementation("androidx.activity:activity-ktx:1.11.0")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
    implementation("androidx.documentfile:documentfile:1.1.0")
    implementation(files("libs/daywire.aar"))
    if (sherpaAar.isFile) {
        implementation(files(sherpaAar))
    }
    testImplementation("junit:junit:4.13.2")
}

val daywireAar = layout.projectDirectory.file("libs/daywire.aar")

tasks.register("requireGoAar") {
    val aarFile = daywireAar.asFile
    doFirst {
        if (!aarFile.isFile) {
            throw GradleException(
                "Missing app/libs/daywire.aar. From the repository root run: ./hack/android-bind.sh",
            )
        }
    }
}

tasks.named("preBuild") {
    dependsOn("requireGoAar")
}
