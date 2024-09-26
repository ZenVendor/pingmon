#! /bin/bash

echo "Moving data to $HOME/pingmon-uninstall"
mkdir $HOME/pingmon-uninstall
mv /var/log/pingmon.log $HOME/pingmon-uninstall/
mv /var/local/pingmon/pingmon.db $HOME/pingmon-uninstall/
mv /usr/local/etc/pingmon.conf $HOME/pingmon-uninstall/

echo "Removing binary and /var/local/pingmon dir"
rm /usr/local/bin/pingmon
rmdir /var/local/pingmon

echo "End."

