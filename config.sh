#!/bin/bash


echo $(which apictl)
apictl version

echo 'Setting up test environment'

apictl remove env -n local 2>/dev/null || true
apictl add env -n local -apim https://localhost:9443

echo 'Logging into local'

apictl login local -u "$USERNAME" -p "$PASSWORD" -k
