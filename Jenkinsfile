pipeline {
    agent any
    environment {
        WSO2_CREDS    = credentials('wso2-publisher-creds')
        WSO2_BASE_URL = 'https://localhost:9443/api/am/publisher/v4'
        PCTL_BIN      = "${WORKSPACE}\\bin"
    }
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        stage('Build pctl') {
            steps {
                bat '''
                    go build -o "%PCTL_BIN%\\pctl.exe" .
                '''
            }
        }
        stage('Verify pctl') {
            steps {
                bat '"%PCTL_BIN%\\pctl.exe" --help > NUL'
            }
        }
        stage('Detect changed policies') {
            steps {
                script {
                    // HEAD~1 doesn't exist on the first commit or a shallow/
                    // single-branch checkout, so fall back to git's "empty
                    // tree" hash (4b825dc...) to diff against nothing, which
                    // yields "everything is new/changed" instead of erroring.
                    def changedFiles = bat(
                        script: '''@echo off
git rev-parse HEAD~1 >NUL 2>&1
if %ERRORLEVEL% EQU 0 (
    git diff --name-only HEAD~1 HEAD -- "policies/*.j2" 2>NUL
) else (
    git diff --name-only 4b825dc642cb6eb9a060e54bf8d69288fbee4904 HEAD -- "policies/*.j2" 2>NUL
)
exit /b 0''',
                        returnStdout: true
                    ).trim()
                    // Joined with spaces (not newlines) so it's safe to drop
                    // straight into a batch command as %CHANGED_POLICIES%
                    env.CHANGED_POLICIES = changedFiles ? changedFiles.readLines().join(' ') : ''
                }
            }
        }
    stage('Lint policies') {
            when {
                expression { env.CHANGED_POLICIES }
            }
            steps {
                bat '''
                    python -m pip install --user --quiet j2lint
                    python -m j2lint %CHANGED_POLICIES%
                '''
            }
        }
        stage('Configure environment') {
            steps {
                bat '''
                    REM "%PCTL_BIN%\\pctl.exe" add-env jenkins --burl "%WSO2_BASE_URL%" --username "%WSO2_CREDS_USR%" --password "%WSO2_CREDS_PSW%"
                    REM "%PCTL_BIN%\\pctl.exe" set-env jenkins
                '''
            }
        }
        stage('Publish changed policies') {
            when {
                expression { env.CHANGED_POLICIES }
            }
            steps {
                script {
                    env.CHANGED_POLICIES.split(' ').each { path ->
                        def policyName = path.tokenize('\\/').last().replace('.j2', '')
                        bat "\"%PCTL_BIN%\\pctl.exe\" publish-policy \"${policyName}\" \"${path}\""
                    }
                }
            }
        }
        stage('Update APIs with new policies') {
            steps {
                // Feeds "1" then "1" into pctl update's two prompts (all
                // policies, then all API Products), same as printf did on Linux
                bat '''
                    (echo 1
echo 1) | "%PCTL_BIN%\\pctl.exe" update
                '''
            }
        }
    }
    post {
        always {
            // vars.go puts the Windows config/logs dir under %USERPROFILE%\.pctl
            // (not ~/.config/pctl like on Linux)
            bat 'type "%USERPROFILE%\\.pctl\\logs\\log.txt" 2>NUL'
        }
        failure {
            echo 'Pipeline failed - check pctl output above.'
        }
    }
}