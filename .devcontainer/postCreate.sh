#!/bin/sh
set -ex

echo 'PS1="\[\e[34m\]\W\[\e[m\] \$ "' >> ~/.bashrc
echo 'PROMPT="\[\e[34m\]%C\[\e[m\] \$ "' >> ~/.zshrc

ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    ARCH=ubuntu_amd64
elif [ "$ARCH" = "aarch64" ]; then
    ARCH=ubuntu_arm64
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/$ARCH/session-manager-plugin.deb" -o "session-manager-plugin.deb"
sudo dpkg -i session-manager-plugin.deb
rm session-manager-plugin.deb
