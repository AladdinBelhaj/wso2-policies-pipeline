import os
from dotenv import load_dotenv 
load_dotenv()

# Environment variables. Change it to the desired URL in .env file
base_url = os.getenv('BASE_URL')