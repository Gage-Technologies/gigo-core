

import os
from tzlocal import get_localzone
import datetime as dt
import jwt as jwt_lib
from datetime import datetime
from requests import get

# specify how long the token will be valid for in the periods below
import pytz

valid_days = 1_000_000
valid_hours = 0
valid_minutes = 0

# optional file for JWT dump if passed
out_file = "/tmp/"

# define path to key file
key_path = "/keys/private.pem"

# read key
private_key = open(key_path, "rb").read()

# generate expiration
expiration = datetime.now().astimezone() + dt.timedelta(days=valid_days, hours=valid_hours, minutes=valid_minutes)

ip = get('https://api.ipify.org').content.decode('utf8')

# form payload
payload = {
    "color_palette": None,
    "email": "",
    "exp": 3088655278,
    "ip": "",
    "name": None,
    "phone": "",
    "render_in_front": None,
    "thumbnail": "/static/user/pfp/",
    "user": "",
    "user_name": "",
    "user_status": 0
}

# create JWT
new_jwt = jwt_lib.encode(payload, private_key, algorithm="RS256")

# if out file passed; dump JWT to file
if out_file:
    with open(out_file, "w+b") as f:
        f.write(new_jwt.encode())

print("##########################################################")
print("INITIALIZATION JWT")
print("IP: ", ip)
print("EXPIRATION: ", expiration.astimezone().strftime("%m/%d/%Y %H:%M:%S"))
print("TOKEN: ", new_jwt)
print("##########################################################")