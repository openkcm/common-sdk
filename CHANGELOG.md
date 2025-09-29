# Changelog

## [1.4.4](https://github.com/openkcm/common-sdk/compare/v1.4.3...v1.4.4) (2025-09-29)


### Bug Fixes

* commonfs fixes and introduced a notifier ([#132](https://github.com/openkcm/common-sdk/issues/132)) ([c81ddea](https://github.com/openkcm/common-sdk/commit/c81ddea4e09c5239f14d1667573e6b00b6df49e1))

## [1.4.3](https://github.com/openkcm/common-sdk/compare/v1.4.2...v1.4.3) (2025-09-26)


### Bug Fixes

* add documentation for common fs and storage ([#128](https://github.com/openkcm/common-sdk/issues/128)) ([bd9d329](https://github.com/openkcm/common-sdk/commit/bd9d32937a7b705d8c7f79113d7ba9b87b5b4d83))

## [1.4.2](https://github.com/openkcm/common-sdk/compare/v1.4.1...v1.4.2) (2025-09-25)


### Bug Fixes

* make generic the key value memory storage ([#126](https://github.com/openkcm/common-sdk/issues/126)) ([6679b97](https://github.com/openkcm/common-sdk/commit/6679b97afa097b8a9d7983fa6d08799c29f5ffa8))

## [1.4.1](https://github.com/openkcm/common-sdk/compare/v1.4.0...v1.4.1) (2025-09-25)


### Bug Fixes

* refactor the common file system watcher ([#124](https://github.com/openkcm/common-sdk/issues/124)) ([8d1dbf4](https://github.com/openkcm/common-sdk/commit/8d1dbf4a547ec07a98f8e0d0d905b3827aab57aa))

## [1.4.0](https://github.com/openkcm/common-sdk/compare/v1.3.0...v1.4.0) (2025-09-22)


### Features

* add filesystem watcher and notify which file was modified over the handlers ([#121](https://github.com/openkcm/common-sdk/issues/121)) ([39b341e](https://github.com/openkcm/common-sdk/commit/39b341e8baa5c1cda0df9652ca7c88e03f183ce3))
* add issuer to clientdata ([#118](https://github.com/openkcm/common-sdk/issues/118)) ([510ca0c](https://github.com/openkcm/common-sdk/commit/510ca0cdfec22ae71ae27f959a9ed438fd5e70e8))


### Bug Fixes

* build information is passed as base64(&lt;encoded value&gt;) ([#120](https://github.com/openkcm/common-sdk/issues/120)) ([e905671](https://github.com/openkcm/common-sdk/commit/e905671f12e1f7ffae0fffb34e82d9fafdb6f84b))

## [1.3.0](https://github.com/openkcm/common-sdk/compare/v1.2.4...v1.3.0) (2025-09-03)


### Features

* Add log level trace ([#106](https://github.com/openkcm/common-sdk/issues/106)) ([e24c3a4](https://github.com/openkcm/common-sdk/commit/e24c3a47d785573d37dda5cdc138f7d3c58acbf4))
* combine load mtls config ([#90](https://github.com/openkcm/common-sdk/issues/90)) ([f9cf635](https://github.com/openkcm/common-sdk/commit/f9cf6355e2157deeccb898dc955afa79569b171f))


### Bug Fixes

* remove grouping for service environment and name ([#101](https://github.com/openkcm/common-sdk/issues/101)) ([d224657](https://github.com/openkcm/common-sdk/commit/d22465758151309e72500f7d2ed740fae1d186eb))

## [1.2.4](https://github.com/openkcm/common-sdk/compare/v1.2.3...v1.2.4) (2025-08-28)


### Bug Fixes

* change the value to reference for couple functions ([#88](https://github.com/openkcm/common-sdk/issues/88)) ([73cd706](https://github.com/openkcm/common-sdk/commit/73cd706bbaaf6e8569e937b0a90f8e26ff7064f1))

## [1.2.3](https://github.com/openkcm/common-sdk/compare/v1.2.2...v1.2.3) (2025-08-27)


### Bug Fixes

* refactor the oauth2 credentials and add the pointers set of func… ([#85](https://github.com/openkcm/common-sdk/issues/85)) ([4ff75d0](https://github.com/openkcm/common-sdk/commit/4ff75d0f0b36d0269cd52e69b8b6b6c04702494b))

## [1.2.2](https://github.com/openkcm/common-sdk/compare/v1.2.1...v1.2.2) (2025-08-21)


### Bug Fixes

* add the oauth2 base url for fetching the tokens ([#81](https://github.com/openkcm/common-sdk/issues/81)) ([0a139be](https://github.com/openkcm/common-sdk/commit/0a139be660ba60995cde72b80a2a0ba278e80575))

## [1.2.1](https://github.com/openkcm/common-sdk/compare/v1.2.0...v1.2.1) (2025-08-21)


### Bug Fixes

* include the mtls into the oauth2 secret type ([#79](https://github.com/openkcm/common-sdk/issues/79)) ([a88d9f8](https://github.com/openkcm/common-sdk/commit/a88d9f8f41beb3995e897f753d45ed6cc690b0b9))

## [1.2.0](https://github.com/openkcm/common-sdk/compare/v1.1.1...v1.2.0) (2025-08-21)


### Features

* add new type of credentials oauth2 based on client id and clien… ([#76](https://github.com/openkcm/common-sdk/issues/76)) ([280d260](https://github.com/openkcm/common-sdk/commit/280d26008c571dee968e60d32769f42d7893b609))


### Bug Fixes

* adjusts the lints ([#77](https://github.com/openkcm/common-sdk/issues/77)) ([0a0906b](https://github.com/openkcm/common-sdk/commit/0a0906bc306ee1a7474719eec915aa625544f7f5))
* stop error on created status ([3f523cd](https://github.com/openkcm/common-sdk/commit/3f523cdba6db1ec59fd6dc6b094e211da6d1821b))
* stop error on created status ([#74](https://github.com/openkcm/common-sdk/issues/74)) ([3f523cd](https://github.com/openkcm/common-sdk/commit/3f523cdba6db1ec59fd6dc6b094e211da6d1821b))

## [1.1.1](https://github.com/openkcm/common-sdk/compare/v1.1.0...v1.1.1) (2025-07-29)


### Bug Fixes

* move the feature gates field at level of application ([#70](https://github.com/openkcm/common-sdk/issues/70)) ([ad36c84](https://github.com/openkcm/common-sdk/commit/ad36c847e1c998113cd948806d574eb30d1ea4c7))

## [1.1.0](https://github.com/openkcm/common-sdk/compare/v1.0.0...v1.1.0) (2025-07-29)


### Features

* add new set of github actions ([#66](https://github.com/openkcm/common-sdk/issues/66)) ([0e7fc7e](https://github.com/openkcm/common-sdk/commit/0e7fc7e2d9e14928668b95a3ed067242ab7aec9e))


### Bug Fixes

* load environment value from env and value fields ([#67](https://github.com/openkcm/common-sdk/issues/67)) ([90ede9d](https://github.com/openkcm/common-sdk/commit/90ede9d2bc93f8b35c3ec7356a7bd4a707e70e61))
