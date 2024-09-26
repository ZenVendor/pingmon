#! /bin/bash

echo "Creating directory /var/local/pingmon"
mkdir /var/local/pingmon 

echo "Creating database file"
cat create.sql | sqlite3 /var/local/pingmon/pingmon.db
chmod 640 /var/local/pingmon/pingmon.db

echo "Copying config to /usr/local/etc/"
cp default.conf /usr/local/etc/pingmon.conf
chmod 640 /usr/local/etc/pingmon.conf

echo "Copying binary to /usr/local/bin"
cp ../test/pingmon /usr/local/bin/
chmod 750 /usr/local/bin/pingmon

echo "Copying service unit to /run/systemd/system"
cp pingmon.service /run/systemd/system/
chmod 640 /run/systemd/system/pingmon.service

systemctl daemon-reload
systemctl enable pingmon.service


echo "End."
