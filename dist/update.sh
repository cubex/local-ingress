#!/usr/bin/env bash

platform='windows'
ext=""
unamestr=`uname`
if [[ "$unamestr" == 'Linux' ]]; then
   platform='linux'
elif [[ "$unamestr" == 'Darwin' ]]; then
   platform='mac'
else
    ext=".exe"
fi

suffix=""
if [[ "$platform" == 'mac' && `uname -m` == 'arm64' ]]; then
   suffix="-arm64"
fi

echo "Downloading Cubex Local-Ingress"
curl -s -o local-ingress$ext https://raw.githubusercontent.com/cubex/local-ingress/master/dist/$platform/local-ingress$suffix$ext
chmod +x local-ingress$ext
echo "Downloaded local-ingress$ext"
