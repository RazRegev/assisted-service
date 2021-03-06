String cron_string = BRANCH_NAME == "master" ? "@hourly" : ""

pipeline {
    agent { label 'centos_worker' }
    triggers { cron(cron_string) }
    environment {
        SERVICE = 'ocpmetal/assisted-service'
        ISO_CREATION = 'assisted-iso-create'
        SLACK_TOKEN = credentials('slack-token')
    }
    options {
      timeout(time: 1, unit: 'HOURS') 
    }

    stages {
        stage('Check environment') {
            steps {
                        sh '''
                            if [ $(minikube status|grep Running)="" ] ; then
                                echo "minikube is not running on $NODE_NAME, failing job BUILD_URL"
                                echo '{"text":"minikube is not running on: ' > minikube_data.txt
                                echo ${NODE_NAME} >> minikube_data.txt
                                echo 'failing job: ' >> minikube_data.txt
                                echo ${BUILD_URL} >> minikube_data.txt
                                echo '"}' >> minikube_data.txt
                                curl -X POST -H 'Content-type: application/json' --data-binary "@minikube_data.txt"  https://hooks.slack.com/services/$SLACK_TOKEN
                                sh "exit 1"
                            fi
                        '''
            }
        }

        stage('clear deployment') {
            steps {
                sh 'docker image prune -a -f'
                sh 'make clear-deployment'
            }
        }

        stage('Build') {
            steps {
                sh '''export PATH=$PATH:/usr/local/go/bin; make build-image build-assisted-iso-generator-image'''
            }
        }

        stage('Deploy for subsystem') {
            steps {
                withCredentials([usernamePassword(credentialsId: 'dockerio_cred', passwordVariable: 'PASS', usernameVariable: 'USER')]) {
                    sh '''docker login docker.io -u $USER -p $PASS'''
                }
                sh '''export PATH=$PATH:/usr/local/go/bin; make jenkins-deploy-for-subsystem'''
                sh '''# Dump pod statuses;kubectl get pods -A'''
            }
        }

        stage('Subsystem-test') {
            steps {
                sh '''export PATH=$PATH:/usr/local/go/bin;make subsystem-run'''
            }
        }

        stage('clear deployment after subsystem test') {
            steps {
                sh 'make clear-deployment'
            }
        }

        stage('publish images on push to master') {
            when {
                branch 'master'
            }

            steps {
                withCredentials([usernamePassword(credentialsId: 'ocpmetal_cred', passwordVariable: 'PASS', usernameVariable: 'USER')]) {
                    sh '''docker login quay.io -u $USER -p $PASS'''
                }

                sh '''docker tag  ${SERVICE} quay.io/ocpmetal/assisted-service:latest'''
                sh '''docker tag  ${SERVICE} quay.io/ocpmetal/assisted-service:${GIT_COMMIT}'''
                sh '''docker push quay.io/ocpmetal/assisted-service:latest'''
                sh '''docker push quay.io/ocpmetal/assisted-service:${GIT_COMMIT}'''

                sh '''docker tag  ${ISO_CREATION} quay.io/ocpmetal/assisted-iso-create:latest'''
                sh '''docker tag  ${ISO_CREATION} quay.io/ocpmetal/assisted-iso-create:${GIT_COMMIT}'''
                sh '''docker push quay.io/ocpmetal/assisted-iso-create:latest'''
                sh '''docker push quay.io/ocpmetal/assisted-iso-create:${GIT_COMMIT}'''
            }
        }
    }

    post {
        failure {
            script {
                if (env.BRANCH_NAME == 'master')
                    sh '''
                           echo '{"text":"Attention! assisted-service master branch subsystem test failed, see: ' > data.txt
                           echo ${BUILD_URL} >> data.txt
                           echo '"}' >> data.txt
                           curl -X POST -H 'Content-type: application/json' --data-binary "@data.txt"  https://hooks.slack.com/services/$SLACK_TOKEN
                    '''
            }

            echo 'Get assisted-service log'
            sh '''
            kubectl get pods -o=custom-columns=NAME:.metadata.name -A | grep assisted-service | xargs -r -I {} sh -c "kubectl logs {} -n  assisted-installer > test_dd.log"
            mv test_dd.log $WORKSPACE/assisted-service.log || true
            '''

            echo 'Get postgres log'
            sh '''kubectl  get pods -o=custom-columns=NAME:.metadata.name -A | grep postgres | xargs -r -I {} sh -c "kubectl logs {} -n  assisted-installer > test_dd.log"
            mv test_dd.log $WORKSPACE/postgres.log || true
            '''

            echo 'Get scality log'
            sh '''kubectl  get pods -o=custom-columns=NAME:.metadata.name -A | grep scality | xargs -r -I {} sh -c "kubectl logs {} -n  assisted-installer > test_dd.log"
            mv test_dd.log $WORKSPACE/scality.log || true
            '''

            echo 'Get createimage log'
            sh '''kubectl  get pods -o=custom-columns=NAME:.metadata.name -A | grep createimage | xargs -r -I {} sh -c "kubectl logs {} -n  assisted-installer > test_dd.log"
            mv test_dd.log $WORKSPACE/createimage.log || true
            '''
        }
    }
}
