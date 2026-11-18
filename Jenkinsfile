@Library("DPMPipelineUtils@1.5") dpmutils
@Library("ru.sbrf.devsecops@master") devsecops

pipeline {
    agent {
        kubernetes {
            cloud "kubernetes2-prom"
            namespace "worker"
            yaml """
              apiVersion: v1
              kind: Pod
              spec:
                imagePullSecrets:
                  - docker-dev.registry-ci.delta.sbrf.ru.pullsecret
                  - docker-release.registry-ci.delta.sbrf.ru.pullsecret
                containers:
                  - name: jnlp
                    image: docker-release.registry-ci.delta.sbrf.ru/ci00757682/ci00757682/jenkins-agent@sha256:028f82cfb852b15a9a6129a0e4f2a7c30f27e0d67d6a7b6b22203375efd370eb
                    tty: true
                  - name: buildkit
                    image: docker-release.registry-ci.delta.sbrf.ru/ci00757682/ci00757682/jenkins-buildkit:0.15.1-rootless@sha256:553d1f72fd248ccc7246abaa6fac5de315ea75467e1067c8f71fbbc1992d9ae9
                    imagePullPolicy: Always
                    args:
                    - --oci-worker-no-process-sandbox
                    tty: true
                  - name: go
                    image: docker-dev.registry-ci.delta.sbrf.ru/ci00757682/ci00757682/golang:1.26.1-se
                    imagePullPolicy: Always
                    command:
                      - sleep
                    args:
                      - infinity
                    securityContext:
                      runAsUser: 1000
            """
            defaultContainer "go"
        }
    }

    environment {
        CREDS_ID = "${scm.userRemoteConfigs?.get(0)?.credentialsId}"
        CI_AS = "CI00766784"
        CI_M = "CI15891918"
        SVC_NAME = "telemetry-dispatcher"

        DOCKER_REGISTRY_DEV = "docker-dev.registry-ci.delta.sbrf.ru"
        DOCKER_REGISTRY_RELEASE = "registry.sigma.sbrf.ru"

        NEXUS_INT = "int.nexus.sigma.sbrf.ru"
        NEXUS_CI = "nexus-ci.delta.sbrf.ru"
        NEXUS_INFRA = "infra.nexus.sigma.sbrf.ru"

        SONAR_DOMAIN = "Delta"
        SONAR_PROJECT = "telemetry-dispatcher"
        SONAR_URL = "https://sonar.delta.sbrf.ru/sonar"
        
        SC_API = "api.sc-ci.sber.ru"
        SC_URL = "https://sc-ci.sber.ru/sc"
        SC_PROJECT = "cip"
        SC_REPO = "telemetry-dispatcher"
    }

    stages {
        stage("Prepare") {
            steps {
                withCredentials(
                    [
                        usernamePassword(
                            credentialsId: env.CREDS_ID,
                            usernameVariable: "USERNAME",
                            passwordVariable: "PASSWORD"
                        )
                    ]
                ) {
                    container("go") {
                        script {
                            sh "goci netrc login -u ${USERNAME} -p ${PASSWORD} ${SC_API} ${NEXUS_INFRA}"
                            env.PREV_MINOR_TAG = sh(script: "goci gitprevious ${BRANCH}", returnStdout: true)
                        }
                    }
                }
            }
        }

        stage("Build & Push OCI Image") {
            steps {
                withCredentials(
                    [
                        usernamePassword(
                            credentialsId: env.CREDS_ID,
                            usernameVariable: "USERNAME",
                            passwordVariable: "PASSWORD"
                        )
                    ]
                ) {
                    container("buildkit") {
                        script {
                            env.IMAGE_NAME = "${CI_AS}/${CI_M}/${SVC_NAME}".toLowerCase()
                            env.IMAGE_TAG = sh(script: "./scripts/get-distrib-version.sh ${BRANCH}", returnStdout: true)
                            env.IMAGE = "${DOCKER_REGISTRY_DEV}/${IMAGE_NAME}:${IMAGE_TAG}"

                            sh '''
                                mkdir -p ~/.docker
                                export DOCKER_AUTH=$(echo -n "${USERNAME}:${PASSWORD}" | base64 -w 0)
                                yq -n -o=json ".auths[\\"${DOCKER_REGISTRY_DEV}\\"].auth = strenv(DOCKER_AUTH)" > ~/.docker/config.json
                                cat ~/.docker/config.json
                            '''

                            sh '''
                                buildctl build \
                                    --frontend dockerfile.v0 \
                                    --local context=. \
                                    --local dockerfile=. \
                                    --opt build-arg:NEXUS_USERNAME=${USERNAME} \
                                    --opt build-arg:NEXUS_PASSWORD=${PASSWORD} \
                                    --output type=image,name=${IMAGE},push=true
                            '''
                        }
                    }
                    container("go") {
                        script {
                            env.IMAGE_HASH = sh(
                                script: 'goci nexus image-hash ${IMAGE} --user=${USERNAME} --password=${PASSWORD}',
                                returnStdout: true
                            )

                            env.OCI_IMAGE_DISTR = "${IMAGE}@sha256:${IMAGE_HASH}".replaceAll(
                                env.DOCKER_REGISTRY_DEV,
                                env.DOCKER_REGISTRY_RELEASE
                            )
                        }
                    }
                } 
            }
        }

        stage("Archive & Publish Helm Chart") {
            steps {
                script {
                    withCredentials(
                        [
                            usernamePassword(
                                credentialsId: env.CREDS_ID,
                                usernameVariable: "USERNAME",
                                passwordVariable: "PASSWORD"
                            )
                        ]
                    ) {
                        container("go") {
                            env.HELM_CHART_DISTR = "https://${NEXUS_INT}/repository/maven-distr/${CI_AS}/${CI_M}/${IMAGE_TAG}/${CI_M}-${IMAGE_TAG}-distrib.tgz"

                            sh '''
                                goci archive \
                                    --wrapdir=chart \
                                    --workdir=deployment/helm \
                                    --out-nexus=${NEXUS_CI}/maven-distr-dev/${CI_AS}/${CI_M}:${IMAGE_TAG} \
                                    -u "${USERNAME}@delta.sbrf.ru" \
                                    -p ${PASSWORD} \
                                    --replace-rules='{"<IMAGE_TAG>":"$IMAGE_TAG","<IMAGE_HASH>":"$IMAGE_HASH"}'
                            ''' 
                        }
                    }
                }
            }
        }

        stage("Run Security Scans") {
            parallel {
                stage("AppSec") {
                    steps {
                        script {
                            def config = readYaml(file: "./devsecops-config.yml")
                            def res = runAppSec(config, env.GIT_URL, env.BRANCH, env.GIT_COMMIT)
                            println(res)
                        }
                    }
                }

                stage("SonarQube") {
                    steps {
                        withCredentials(
                            [
                                usernamePassword(
                                    credentialsId: env.CREDS_ID,
                                    usernameVariable: "USERNAME",
                                    passwordVariable: "PASSWORD"
                                )
                            ]
                        ) {
                            withSonarQubeEnv(installationName: "SonarQube${env.SONAR_DOMAIN}") {
                                script {
                                    sh '''
                                        sonar-scanner \
                                            -Dsonar.branch.name=${BRANCH} \
                                            -Dsonar.sourceEncoding=UTF-8 \
                                            -Dsonar.scm.provider=git \
                                            -Dsonar.sources=. \
                                            -Dsonar.projectKey=${SONAR_PROJECT} \
                                            -Dsonar.host.url=${SONAR_URL} \
                                            -Dsonar.login=${USERNAME} \
                                            -Dsonar.password=${PASSWORD}
                                    '''
                                }
                            }
                            script {
                                def qs = waitForQualityGate()
                                if (qs.status != "OK") {
                                    error "SonarQube failed: ${qs.status}"
                                }
                            }
                        }
                    }
                }
            }
        }
    
        stage("Create & Upload Release Notes") {
            steps {
                script {
                    releaseNotes()
                        .setNexusUrl("https://${env.NEXUS_CI}/")
                        .setArtifactRepository("maven-distr")
                        .setNexusCredsId(env.CREDS_ID)
                        .setGroup(env.CI_AS)
                        .setArtifact(env.CI_M)
                        .setVersion(env.IMAGE_TAG)
                        .addRepoViaBitbucketAPI(
                            env.SC_PROJECT,
                            env.SC_REPO,
                            env.SC_URL,
                            env.CREDS_ID,
                            true,
                            env.PREV_MINOR_TAG,
                            env.GIT_COMMIT
                        )
                        .createAndUploadReleaseNotes()

                    releaseNotes()
                        .setNexusUrl("https://${env.NEXUS_CI}/")
                        .setArtifactRepository("registry")
                        .setGroup(env.CI_AS)
                        .setArtifact("${env.CI_M}/${env.SVC_NAME}")
                        .setVersion(env.IMAGE_TAG)
                        .setQgmCredsId(env.CREDS_ID)
                        .addRepoViaBitbucketAPI(
                            env.SC_PROJECT,
                            env.SC_REPO,
                            env.SC_URL,
                            env.CREDS_ID,
                            true,
                            env.PREV_MINOR_TAG,
                            env.GIT_COMMIT
                        )
                        .createAndUploadReleaseNotes()

                    println("${env.HELM_CHART_DISTR};${env.OCI_IMAGE_DISTR}")
                }
            }
        }
    }
}