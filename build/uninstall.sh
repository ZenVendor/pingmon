#! /bin/bash

echo "Disabling service"
systemctl stop pingmon.service
systemctl disable pingmon.service

echo "Moving data to $HOME/pingmon-uninstall"
mkdir $HOME/pingmon-uninstall
mv /var/log/pingmon.log $HOME/pingmon-uninstall/
mv /var/local/pingmon/pingmon.db $HOME/pingmon-uninstall/
mv /usr/local/etc/pingmon.conf $HOME/pingmon-uninstall/

echo "Removing binary and /var/local/pingmon dir"
rm /usr/local/bin/pingmon
rm /run/systemd/system/pingmon.service
rmdir /var/local/pingmon


echo "End."

