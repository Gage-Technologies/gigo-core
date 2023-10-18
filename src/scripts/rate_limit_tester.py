import threading
import time

import requests
from joblib import Parallel, delayed


lock = threading.Lock()
hit = False


def _caller():
    global hit

    for _ in range(10):
        r = requests.get("http://gigo.gage.intranet/healthz")
        print(r.status_code)
        if r.status_code == 429:
            with lock:
                hit = True
            return
        time.sleep(.2)


Parallel(n_jobs=30, prefer="threads")(delayed(_caller)() for _ in range(30))

print("Hit: ", hit)
