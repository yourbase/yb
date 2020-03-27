package "fingo_app" { 
  buildpack "go" { version = "1.13.1" }
  buildpack "flutter" { version = "1.0" }

  setup { 
  }

  build {
  }

  test {
    commands = [
      "./tools/dartfmt-check",
      "./tools/flutter-analyze",
      "./tools/run-dart-tests",
      "./tools/install-golangci-lint.sh",
      "./tools/run-go-tests",
      "./tools/eslint"
    ]
  }
}

package "fingo_android" {
    buildpack "android" { version = "latest" }
    buildpack "java" { version = "8.202.08" }
    
    build {
      commands = [
        "sdkmanager --install tools",
        "sdkmanager --install platform-tools",
        "sdkmanager --install build-tools;28.0.3",
        "sdkmanager --install platforms;android-28",
        "sdkmanager --install system-images;android-28;default;x86_64",
        "apt-get update",
        "apt-get install -y software-properties-common",
        "add-apt-repository -y ppa:ubuntu-toolchain-r/test",
        "apt-get update",
        "apt-get install -y lib32stdc++6"
      ]
    }

    test {
    }
}

package "fingo_android_production" {
    depends_on = ["package.fingo_android"]

    build {
        command = "./android/publish.sh master"
    }
    
    ci { 
        when {
            branch = "master"
            tagged = true
        }
    }
}

package "fingo_android_alpha" {
    depends_on = ["package.fingo_android"]

    build {
        command = "./android/publish.sh alpha"
    }

    ci { 
        when {
            not { branch = "master" }
            tagged = false
        }
    }
} 

package "fingo_ios_production" {
    build {
        commands = [
            "cd lumina/flutter/fingo_app/",
            "./ios/install_deps.sh",
            "./ios/publish.sh master"
        ]
    }

    ci {
        when {
            tagged = true
        }
    }
}

package "fingo_ios_alpha" { 
    build {
        commands = [
            "cd lumina/flutter/fingo_app/",
            "./ios/install_deps.sh",
            "./ios/publish.sh master"
        ]
    }

    ci {
        when {
            branch = "master"
            tagged = false
        }
    }
}
