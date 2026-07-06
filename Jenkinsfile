pipeline {
    agent any
    environment {
        CI = 'true'
        API = './PizzaShackAPI-1.0.0'
    }

    stages {
        stage('Setup APIM Environments'){
            steps{
                withCredentials([usernamePassword(credentialsId: 'apim', usernameVariable: 'USERNAME', passwordVariable: 'PASSWORD')]){
                   sh './.config.sh'
                }
            }
        }
    }
}