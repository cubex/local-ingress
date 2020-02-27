#!/bin/bash

PLATFORMS="darwin/amd64"
PLATFORMS="$PLATFORMS windows/amd64 windows/386"
PLATFORMS="$PLATFORMS linux/amd64 linux/386"

type setopt >/dev/null 2>&1

SCRIPT_NAME=`basename "$0"`
FAILURES=""
SOURCE_FILE=`echo $@ | sed 's/\.go//'`
CURRENT_DIRECTORY=${PWD##*/}
OUTPUT="dist"

for PLATFORM in $PLATFORMS; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}

  GOOSDISP=${GOOS}
  if [[ "${GOOSDISP}" == "darwin" ]]; then GOOSDISP="mac"; fi

  BIN_FILENAME="${OUTPUT}/${GOOSDISP}/local-ingress"
  if [[ "${GOARCH}" != "amd64" ]]; then BIN_FILENAME="${BIN_FILENAME}-${GOARCH}"; fi
  if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
  CMD="GOOS=${GOOS} GOARCH=${GOARCH} go build -o ${BIN_FILENAME} $@"
  echo "${CMD}"
  eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
done

# eval errors
if [[ "${FAILURES}" != "" ]]; then
  echo ""
  echo "${SCRIPT_NAME} failed on: ${FAILURES}"
  exit 1
fi