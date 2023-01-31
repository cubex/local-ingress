#!/usr/bin/env bash

platform='windows'
ext=""
unamestr=$(uname)
if [[ "$unamestr" == 'Linux' ]]; then
  platform='linux'
elif [[ "$unamestr" == 'Darwin' ]]; then
  platform='mac'
else
  ext=".exe"
fi

echo "Downloading Cubex Local-Ingress"
curl -s -O https://raw.githubusercontent.com/cubex/local-ingress/master/dist/bin/$platform-local-ingress$ext
chmod +x local-ingress$ext
echo "Downloaded local-ingress$ext"
