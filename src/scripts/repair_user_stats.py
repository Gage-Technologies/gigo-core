from typing import Tuple
import datetime
import os
import tqdm
import mysql.connector
from joblib import Parallel, delayed

# Configure database connection
config = {
    'user': os.environ["GIGO_DB_USER"],
    'password': os.environ["GIGO_DB_PASS"],
    'host': os.environ["GIGO_DB_HOST"],
    'port': int(os.environ["GIGO_DB_PORT"]),
    'database': os.environ["GIGO_DB_DATABASE"],
    'raise_on_warnings': True
}

cnx = mysql.connector.connect(**config)


cnx.start_transaction()
with cnx.cursor() as cursor:
    cursor.execute('select _id from users')
    user_ids = [x[0] for x in cursor.fetchall()]
cnx.commit()

pbar = tqdm.tqdm(total=len(user_ids), desc="Deleting Duplicate Open Rows")
for id in user_ids:
    cnx.start_transaction()
    with cnx.cursor(dictionary=True) as cursor:
        cursor.execute(
            """
            DELETE FROM user_stats
            WHERE 
                _id != (
                    SELECT MIN(_id)
                    FROM user_stats
                    WHERE closed = 0 AND user_id = %s
                )
            AND closed = 0 AND user_id = %s;
            """,
            (id, id)
        )
    cnx.commit()
    pbar.update(1)


cnx = mysql.connector.connect(**config)
cnx.start_transaction()

# SQL query to delete duplicates
delete_query = """
DELETE u1 FROM user_stats u1
INNER JOIN (
    SELECT MIN(_id) as min_id, user_id, date
    FROM user_stats
    GROUP BY user_id, date
) u2 ON u1.user_id = u2.user_id AND u1.date = u2.date
WHERE u1._id > u2.min_id;
"""

with cnx.cursor() as cursor:
    cursor.execute(delete_query)
    cnx.commit()

cnx.close()
