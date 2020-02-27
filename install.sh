#!/usr/bin/env bash

echo "Downloading Cubex Local Ingress"

FILE="config.yaml"
if [ -f "$FILE" ]; then
    echo "$FILE exist, skipping"
else
    echo "$FILE does not exist, downloading from dist"
    curl -s -O https://raw.githubusercontent.com/cubex/local-ingress/master/dist/config.yaml
fi


FILE="update.sh"
if [ -f "$FILE" ]; then
    echo "$FILE exist, skipping"
else
    echo "$FILE does not exist, downloading from dist"
    curl -s -O https://raw.githubusercontent.com/cubex/local-ingress/master/dist/update.sh
    chmod +x update.sh
fi

./update.sh