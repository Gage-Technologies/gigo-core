echo -e "Initializing Runner...\n"
git config --global url."https://${GITHUB_USER_NAME}:${GITHUB_BUILD_TOKEN}@github.com".insteadOf "https://github.com"
sudo apt update && sudo apt install mycli -y

# Loop to wait for TiDB to come online
echo "Waiting for TiDB to come online..."
for i in $(seq 1 600); do
  mycli -u root -h gigo-dev-tidb -P 4000 -e "/* ping */ SELECT 1;" > /dev/null 2>&1
  exit_code=$?

  if [ "$exit_code" == "0" ]; then
    echo "TiDB is online!"
    break
  else
    echo "Attempt #$i/600: TiDB is not online yet. Retrying in 1 second..."
    sleep 1s
  fi
done

if [ "$exit_code" != "0" ]; then
  echo "TiDB did not come online after 600 attempts. Exiting..."
  exit 1
fi

echo -e "Initializing TiDB User...\n"
mycli -u root -h gigo-dev-tidb -P 4000 -e "SET GLOBAL tidb_multi_statement_mode='ON'"
mycli -u root -h gigo-dev-tidb -P 4000 -e "SET GLOBAL sql_mode=(SELECT REPLACE(@@sql_mode,'ONLY_FULL_GROUP_BY',''));"
mycli -u root -h gigo-dev-tidb -P 4000 -e "CREATE USER 'gigo-dev'@'%' IDENTIFIED BY 'gigo-dev';"
mycli -u root -h gigo-dev-tidb -P 4000 -e "GRANT ALL PRIVILEGES ON *.* TO 'gigo-dev'@'%' WITH GRANT OPTION;"
mycli -u root -h gigo-dev-tidb -P 4000 -e "FLUSH PRIVILEGES;"

echo -e "Starting Runner...\n"
while true; do
   sleep 1d
done

echo -e "Runner Exiting...\n"