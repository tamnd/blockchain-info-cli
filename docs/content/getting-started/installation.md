---
title: "Installation"
description: "Install blockchain-info from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/blockchain-info-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `blockchain-info` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/blockchain-info-cli/cmd/blockchain-info@latest
```

That puts `blockchain-info` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/blockchain-info-cli
cd blockchain-info-cli
make build        # produces ./bin/blockchain-info
./bin/blockchain-info version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/blockchain-info:latest --help
```

## Checking the install

```bash
blockchain-info version
```

prints the version and exits.
