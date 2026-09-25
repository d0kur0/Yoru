## [1.3.1](https://github.com/d0kur0/Yoru/compare/v1.3.0...v1.3.1) (2026-09-25)

### Bug Fixes

* apply live proxy edits and clarify connection checks ([832b2fb](https://github.com/d0kur0/Yoru/commit/832b2fb3ccd58e47ef394ff6c5727a784d1f5d74))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

## [1.3.0](https://github.com/d0kur0/Yoru/compare/v1.2.1...v1.3.0) (2026-09-25)

### Features

* check and install application updates ([1956efd](https://github.com/d0kur0/Yoru/commit/1956efdeb93a5e7a4daa4c5704ac35f416be411c))

### Bug Fixes

* authenticate bundled core metadata in CI ([4549237](https://github.com/d0kur0/Yoru/commit/454923793092c2b90e57fee6d5706406d9773076))
* preserve TUN when switching VPN servers ([f829623](https://github.com/d0kur0/Yoru/commit/f829623e85cb14f807eb5c6d8ebdfb2fd5d65e9e))
* resolve NSIS path before Windows packaging ([cc6d820](https://github.com/d0kur0/Yoru/commit/cc6d82015d3352016579d0ed8d1b34b586f97f01))
* retry incomplete NSIS installation in CI ([f659b83](https://github.com/d0kur0/Yoru/commit/f659b83c12c0ae04e237b92fbcf407f217d3f4b0))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

## [1.2.1](https://github.com/d0kur0/Yoru/compare/v1.2.0...v1.2.1) (2026-09-22)

### Bug Fixes

* encode Windows autostart task as UTF-16 ([1a48665](https://github.com/d0kur0/Yoru/commit/1a48665f49a17a1db230ea958bd2b5ea42a3d8a0))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

## [1.2.0](https://github.com/d0kur0/Yoru/compare/v1.1.1...v1.2.0) (2026-09-22)

### Features

* reload routing live and organize rule editors ([7f5a0ed](https://github.com/d0kur0/Yoru/commit/7f5a0ed24aee5e0273723c53ba96546216457a0c))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

## [1.1.1](https://github.com/d0kur0/Yoru/compare/v1.1.0...v1.1.1) (2026-09-19)

### Bug Fixes

* elevate Windows app before startup ([bc168cd](https://github.com/d0kur0/Yoru/commit/bc168cd0496dbd4fa4000318192cb73b08ea7892))


### Installers

Windows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.

These builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.

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
