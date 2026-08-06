# Changelog

All notable changes to Diskforge are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Release Please maintains this file from Conventional Commits. Contributors do
not edit release sections manually.

## [0.1.1](https://github.com/ioplane/diskforge/compare/v0.1.0...v0.1.1) (2026-08-06)


### Bug fixes

* **release:** mount Git metadata for publishing ([#9](https://github.com/ioplane/diskforge/issues/9)) ([4bb94bf](https://github.com/ioplane/diskforge/commit/4bb94bfadd16aa5435e78d08e4dab6ec2422f0a5))

## 0.1.0 (2026-08-06)


### Features

* **api:** expose reusable diskforge engine ([e2a841e](https://github.com/ioplane/diskforge/commit/e2a841ed5fa4c9c1606431a9c0fa159644549b48))
* **cli:** expose guarded disk imaging commands ([8017fa9](https://github.com/ioplane/diskforge/commit/8017fa9813a00daf181d6c5bf26a3d0314deefc4))
* **contract:** enforce portable artifact naming ([2e7d22e](https://github.com/ioplane/diskforge/commit/2e7d22e35f097edd5048192a8fd8f47cadb733f8))
* **image:** stream only complete verified sources ([e576027](https://github.com/ioplane/diskforge/commit/e57602723d3939ca762989e1f7c14362652b1e6b))
* **linux:** enforce guarded whole-disk execution ([4c6126f](https://github.com/ioplane/diskforge/commit/4c6126fe649b46636dccfe762802d4f12a39cc7b))
* **policy:** enforce immutable disk safety decisions ([4114fc6](https://github.com/ioplane/diskforge/commit/4114fc6fdf721ad965905b7c7bbb29f72571e96c))


### Bug fixes

* **dependabot:** exclude Python prerelease range ([1ca3944](https://github.com/ioplane/diskforge/commit/1ca3944ad6dbe277175e22cab5921836acd79ede))
* **dependabot:** use valid Docker version requirement ([4927759](https://github.com/ioplane/diskforge/commit/4927759dde032630b6ce519d0ce121e4bb75adaa))
* **linux:** resolve file-backed swap devices ([#4](https://github.com/ioplane/diskforge/issues/4)) ([6fcdd88](https://github.com/ioplane/diskforge/commit/6fcdd88f7b37b93475867a298aab208af40a3f40))
* **release:** start versioning at v0.1.0 ([1062bf0](https://github.com/ioplane/diskforge/commit/1062bf0b06735285face38d8214679df8f65705b))
* **test:** tolerate legacy generated output ([#7](https://github.com/ioplane/diskforge/issues/7)) ([26249ec](https://github.com/ioplane/diskforge/commit/26249ecbf43e58de8b3c341fc0a1034a21a33c3f))


### Refactoring

* **repository:** adopt Go-native project layout ([#6](https://github.com/ioplane/diskforge/issues/6)) ([daeec61](https://github.com/ioplane/diskforge/commit/daeec61a31a4578eb2f4e1c6aca145583daa1e92))


### Documentation

* **architecture:** define Go-native repository layout ([#5](https://github.com/ioplane/diskforge/issues/5)) ([f0c6450](https://github.com/ioplane/diskforge/commit/f0c64502b13fd4a93e86f55bab249ad5ed346b70))
* **architecture:** define repository and release contract ([e1842f2](https://github.com/ioplane/diskforge/commit/e1842f2ac1274662aee05fadd74a5ead5c7fab43))
* **governance:** align protected branch policy ([83a88ce](https://github.com/ioplane/diskforge/commit/83a88ce20cce17c1f092288980879178e7ebe6e7))
* **governance:** document workflow token policy ([680d201](https://github.com/ioplane/diskforge/commit/680d201d4ac8ef064b4bcf7df119f72b689a39ae))
* **governance:** establish public project standards ([6dc4f34](https://github.com/ioplane/diskforge/commit/6dc4f34f0980487aaa29ed401a653c3619d7e832))


### Tests and evidence

* **integration:** verify guarded loop device write ([2436b83](https://github.com/ioplane/diskforge/commit/2436b839df53c6125e91ac48d3b1d7e4885325fa))


### Build

* **container:** add binary inspection tooling ([2abd59d](https://github.com/ioplane/diskforge/commit/2abd59de452b1b38c0dbc4699609ebd3120a2cb7))
* **container:** establish pinned development environment ([5eebb38](https://github.com/ioplane/diskforge/commit/5eebb385268e6796a9ef1575cffde8ebb08df51b))
* **docs:** enforce documentation quality gates ([3493bed](https://github.com/ioplane/diskforge/commit/3493bede59449d1089b59bdd2ffe6b529c23b405))
* **lint:** enforce comprehensive static analysis ([a4f0b30](https://github.com/ioplane/diskforge/commit/a4f0b309aef7b9b5df9120e905e2f53d4c69aaaf))

## [Unreleased]

[Unreleased]: https://github.com/ioplane/diskforge/compare/v0.1.0...HEAD
