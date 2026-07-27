pipeline {
    agent any

    triggers {
        githubPush()
    }

    environment {
        IMAGE_BASE     = 'ayyashdecom/aimoc-be'
        REPO           = 'https://github.com/Decom-Feno-Mahaka/aimoc-brown-canyon-backend.git'
        CRED_ID        = 'github-credentials'
        DOCKER_CRED_ID = 'dockerhub-credentials'
        DISCORD_WEBHOOK = 'https://discord.com/api/webhooks/1015454032132833443/GvU4rj-MX84Sxd9F7LUEpwYdw8JutLlWnl71P_ycuwgYfUkt0KaNGWdDrorlvs_eCcsj'
    }

    stages {
        stage('Detect Branch') {
            steps {
                script {
                    def branchName = (env.GIT_BRANCH ?: env.BRANCH_NAME ?: 'main').replaceAll('origin/', '')
                    if (branchName == 'development') {
                        env.ENV_TYPE = 'staging'
                        env.IMAGE    = "${IMAGE_BASE}:staging"
                        env.BRANCH   = 'development'
                        env.APP_URL  = 'https://aimoc-staging.golfscore.co.id'
                    } else {
                        env.ENV_TYPE = 'production'
                        env.IMAGE    = "${IMAGE_BASE}:latest"
                        env.BRANCH   = 'main'
                        env.APP_URL  = 'https://aimoc.golfscore.co.id'
                    }
                }
            }
        }

        stage('Clone Repo') {
            steps {
                script {
                    git branch: env.BRANCH, credentialsId: "${CRED_ID}", url: "${REPO}"
                }
            }
        }

        stage('Build Image') {
            steps {
                sh "docker build -t ${env.IMAGE} --no-cache ."
            }
        }

        stage('Push Image') {
            steps {
                withCredentials([usernamePassword(credentialsId: "${DOCKER_CRED_ID}",
                    usernameVariable: 'DOCKER_USER', passwordVariable: 'DOCKER_PASS')]) {
                    sh 'echo $DOCKER_PASS | docker login -u $DOCKER_USER --password-stdin'
                    sh "docker push ${env.IMAGE}"
                }
            }
        }

        stage('Cleanup') {
            steps {
                sh "docker rmi ${env.IMAGE} || true"
                sh "docker image prune -f || true"
            }
        }
    }

    post {
        success {
            node('') {
                sh "curl -s -X POST \"${DISCORD_WEBHOOK}\" -H \"Content-Type: application/json\" -d \'{\"content\": \"✅ **aimoc-be** build sukses [${env.ENV_TYPE}] ${env.APP_URL}\"}\'"
                cleanWs()
            }
        }
        failure {
            node('') {
                sh "curl -s -X POST \"${DISCORD_WEBHOOK}\" -H \"Content-Type: application/json\" -d \'{\"content\": \"❌ **aimoc-be** build GAGAL [${env.ENV_TYPE}]\"}\'"
                cleanWs()
            }
        }
    }
}
