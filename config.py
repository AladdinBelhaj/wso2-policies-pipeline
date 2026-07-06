import subprocess


username=${USERNAME}
password=${PASSWORD}

print('Checking apictl installation')
subprocess.run(['apictl', 'version'])

print('Setting up test environment')

subprocess.run(['apictl', 'remove', 'env', '-n', 'local'], stderr=subprocess.DEVNULL, stdout=subprocess.DEVNULL)
subprocess.run(['apictl', 'add', 'env', '-n', 'local', '-apim', 'https://localhost:9443'])

print('Logging into local')

subprocess.run(['apictl', 'login', 'local', '-u', username, '-p', password, '-k'])
