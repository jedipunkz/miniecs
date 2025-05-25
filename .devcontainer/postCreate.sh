#!/bin/sh
set -ex

echo 'PS1="\[\e[34m\]\W\[\e[m\] \$ "' >> ~/.bashrc
echo 'PROMPT="\[\e[34m\]%C\[\e[m\] \$ "' >> ~/.zshrc
