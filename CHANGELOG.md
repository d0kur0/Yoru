## [1.1.0](https://github.com/d0kur0/Yoru/compare/v1.0.0...v1.1.0) (2026-09-19)

### Features

* request Windows admin rights once for TUN instead of every launch ([26d43c5](https://github.com/d0kur0/Yoru/commit/26d43c5f860073d2fcd528fe43929235486b22ba))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

## 1.0.0 (2026-09-18)

### Features

* add portable route sets and live VPN settings ([d4e370e](https://github.com/d0kur0/Yoru/commit/d4e370e63714cfe2e3b729be30c4d443fd434d70))
* introduce Yoru desktop client ([9161b33](https://github.com/d0kur0/Yoru/commit/9161b337b0970be7c9f1fb2cf1ced5a47f83b9c5))

### Bug Fixes

* serialize go mod tidy before npm ci to stop packaging race ([7f35c63](https://github.com/d0kur0/Yoru/commit/7f35c63e0e3a3c852f8a2285c13a615967fd26c7))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.
