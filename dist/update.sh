#!/usr/bin/env bash

ext=""
case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) os=windows; ext=".exe" ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  i386|i686) arch=386 ;;
  *) arch=amd64 ;;
esac

echo "Downloading Cubex Local-Ingress ($os/$arch)"
curl -sfL --proto =https --proto-redir =https -o "local-ingress$ext" "https://github.com/cubex/local-ingress/releases/latest/download/local-ingress-$os-$arch$ext" || {
  echo "Download failed"
  exit 1
}
chmod +x "local-ingress$ext"
echo "Downloaded local-ingress$ext"
