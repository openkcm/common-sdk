# Changelog

## [1.10.0](https://github.com/openkcm/common-sdk/compare/v1.9.0...v1.10.0) (2026-01-13)


### Features

* add jwks client  ([6535720](https://github.com/openkcm/common-sdk/commit/65357206e1ab0cbe54aa3e53c7e56377928f0a7a))
* add jwks_provider ([#219](https://github.com/openkcm/common-sdk/issues/219)) ([8058e9f](https://github.com/openkcm/common-sdk/commit/8058e9f2aeac24fd92c872291a6d606f36a46b4f))
* add validator for x5c  ([00df734](https://github.com/openkcm/common-sdk/commit/00df734d55b245b74945865cc02ef2a12fef54c4))

## [1.9.0](https://github.com/openkcm/common-sdk/compare/v1.8.0...v1.9.0) (2025-12-19)


### Features

* add jwk builder and http endpoint  ([9bb33cd](https://github.com/openkcm/common-sdk/commit/9bb33cda48efaf4d1fc9ea96e601623e7eecb3d8))
* **audit:** enhance CMK events with systemID field ([#209](https://github.com/openkcm/common-sdk/issues/209)) ([a78e030](https://github.com/openkcm/common-sdk/commit/a78e030997970d52574076ca8e9ee1d901a94ce0))
* **audit:** extend unauthorized event ([#198](https://github.com/openkcm/common-sdk/issues/198)) ([ec356a1](https://github.com/openkcm/common-sdk/commit/ec356a1fe9717c9898ba17351381555c369874da))


### Bug Fixes

* adjust logs of the status server; add a new function helping to set up the base of server ([#184](https://github.com/openkcm/common-sdk/issues/184)) ([d8dde2e](https://github.com/openkcm/common-sdk/commit/d8dde2e14430bccc6c20be291a6f8134ec292993))
* **audit:** add resource and action to properties and refactor ([#199](https://github.com/openkcm/common-sdk/issues/199)) ([83ec4e1](https://github.com/openkcm/common-sdk/commit/83ec4e1f64249546c9369794eed0b5e9b25b4d63))
* **audit:** delete systemID from CMK fields ([#214](https://github.com/openkcm/common-sdk/issues/214)) ([11f5cea](https://github.com/openkcm/common-sdk/commit/11f5cea63dde6567993f5b58aad0116b91d03f68))

## [1.8.0](https://github.com/openkcm/common-sdk/compare/v1.7.0...v1.8.0) (2025-12-03)


### Features

* csrf ([#190](https://github.com/openkcm/common-sdk/issues/190)) ([9c4ba9c](https://github.com/openkcm/common-sdk/commit/9c4ba9c804476eb7378c2b6335983196b7a274d0))

## [1.7.0](https://github.com/openkcm/common-sdk/compare/v1.6.2...v1.7.0) (2025-12-02)


### Features

* add fingerprint package ([#185](https://github.com/openkcm/common-sdk/issues/185)) ([371564f](https://github.com/openkcm/common-sdk/commit/371564f7495f386a5b02e298dbb1eac07c6deb58))
* add jwtsigning package with signing and verification logic ([#168](https://github.com/openkcm/common-sdk/issues/168)) ([38e8ae1](https://github.com/openkcm/common-sdk/commit/38e8ae1ab1c3a7eec0df83096ac13bda99a58d35))
* new audit events for CMK detachment and CMK tenant deletion ([92980a0](https://github.com/openkcm/common-sdk/commit/92980a0f420e8162c216284d531cdda2a51dc291)), closes [#186](https://github.com/openkcm/common-sdk/issues/186)

## [1.6.2](https://github.com/openkcm/common-sdk/compare/v1.6.1...v1.6.2) (2025-11-26)


### Bug Fixes

* Fix BuildInfo yaml serialization ([#179](https://github.com/openkcm/common-sdk/issues/179)) ([d82c67b](https://github.com/openkcm/common-sdk/commit/d82c67b76cca1d57063e71265e096b584e35a8e7))
* introduce a new creation http client using the oauth2 ([#177](https://github.com/openkcm/common-sdk/issues/177)) ([cd5c71b](https://github.com/openkcm/common-sdk/commit/cd5c71bcfcde05a59b6480073a2d9a8e4c227fc1))
* refactor the mtls loading ([#180](https://github.com/openkcm/common-sdk/issues/180)) ([09223a3](https://github.com/openkcm/common-sdk/commit/09223a3469ff0240693b735bd690fd7ec347c99c))
* updated different codes to comply with lints ([#178](https://github.com/openkcm/common-sdk/issues/178)) ([2f10e70](https://github.com/openkcm/common-sdk/commit/2f10e70cd9a622a0c3e5e9fa6c06db8ae414554a))

## [1.6.1](https://github.com/openkcm/common-sdk/compare/v1.6.0...v1.6.1) (2025-11-24)


### Bug Fixes

* **deps:** bump github.com/samber/oops from 1.19.3 to 1.19.4 ([#166](https://github.com/openkcm/common-sdk/issues/166)) ([756023b](https://github.com/openkcm/common-sdk/commit/756023b4e8a38e08772ac82be38aaad0d3d06d40))
* **deps:** bump github.com/samber/slog-multi from 1.5.0 to 1.6.0 ([#167](https://github.com/openkcm/common-sdk/issues/167)) ([d1debc4](https://github.com/openkcm/common-sdk/commit/d1debc48a2865505c426cf2deebf97f90706e462))
* **deps:** bump go.opentelemetry.io/collector/pdata from 1.45.0 to 1.46.0 ([#165](https://github.com/openkcm/common-sdk/issues/165)) ([a2ca7fc](https://github.com/openkcm/common-sdk/commit/a2ca7fc550834897fd113138dc45de518aad14e2))
* **deps:** bump google.golang.org/grpc from 1.76.0 to 1.77.0 ([#170](https://github.com/openkcm/common-sdk/issues/170)) ([2ff7df3](https://github.com/openkcm/common-sdk/commit/2ff7df3db4bfdb582f589d2864b451568565fe55))
* include components to BuildInfo for version endpoint ([#173](https://github.com/openkcm/common-sdk/issues/173)) ([eed00e4](https://github.com/openkcm/common-sdk/commit/eed00e4ab32a2c20436f0ef0d5350f59a33285c7))
* inject as build information the decoded value if applicable ([#169](https://github.com/openkcm/common-sdk/issues/169)) ([f064951](https://github.com/openkcm/common-sdk/commit/f0649518655999bea38d5512b8e93237b166de98))
* retract v2 ([#175](https://github.com/openkcm/common-sdk/issues/175)) ([e11231d](https://github.com/openkcm/common-sdk/commit/e11231de094346fb18623422a9b7358b9b10949a))

## [1.6.0](https://github.com/openkcm/common-sdk/compare/v1.5.2...v1.6.0) (2025-11-10)


### ⚠ BREAKING CHANGES

* **auth:** redesign client data struct ([#158](https://github.com/openkcm/common-sdk/issues/158))


### Miscellaneous Chores

* reset version to 1.6.0 ([adab21a](https://github.com/openkcm/common-sdk/commit/adab21a72f74b2d56c549eb0d1714af843f234e9))

## [1.5.2](https://github.com/openkcm/common-sdk/compare/v1.5.1...v1.5.2) (2025-10-30)


### Bug Fixes

* race conditions ([#153](https://github.com/openkcm/common-sdk/issues/153)) ([2c08833](https://github.com/openkcm/common-sdk/commit/2c08833931e9f2383034b95e6b1dfe5c34f214b9))
* switch defaults library ([#154](https://github.com/openkcm/common-sdk/issues/154)) ([4b9dd38](https://github.com/openkcm/common-sdk/commit/4b9dd3856c715260189d511e8182c406fcd16913))

## [1.5.1](https://github.com/openkcm/common-sdk/compare/v1.5.0...v1.5.1) (2025-10-29)


### Bug Fixes

* add back the loading defaults from fields tags ([#151](https://github.com/openkcm/common-sdk/issues/151)) ([8065e68](https://github.com/openkcm/common-sdk/commit/8065e68632ad2e23bda0e579f13cbc83ca37040c))

## [1.5.0](https://github.com/openkcm/common-sdk/compare/v1.4.7...v1.5.0) (2025-10-28)


### Features

* add audit log events cmkAvailable and cmkUnavailable ([#147](https://github.com/openkcm/common-sdk/issues/147)) ([a56f745](https://github.com/openkcm/common-sdk/commit/a56f7452fcaf0737596c57f32bbf8e586081e975))
* general HTTP client support ([#148](https://github.com/openkcm/common-sdk/issues/148)) ([12349ad](https://github.com/openkcm/common-sdk/commit/12349ad087ba4661b4e401d87c99158bdb79ed38))


### Bug Fixes

* **deps:** bump go.opentelemetry.io/collector/pdata from 1.43.0 to 1.44.0 ([#146](https://github.com/openkcm/common-sdk/issues/146)) ([5fa3bc3](https://github.com/openkcm/common-sdk/commit/5fa3bc309d214828937e863e5facf5be26de7228))
* Remove telemetry url config for gRPC exporters ([#149](https://github.com/openkcm/common-sdk/issues/149)) ([9ff68dd](https://github.com/openkcm/common-sdk/commit/9ff68ddbd68500f84eea3daa5ff18ef1db6d35db))

## [1.4.7](https://github.com/openkcm/common-sdk/compare/v1.4.6...v1.4.7) (2025-10-10)


### Bug Fixes

* assigning nil to chan that is still in use ([#143](https://github.com/openkcm/common-sdk/issues/143)) ([1db0043](https://github.com/openkcm/common-sdk/commit/1db0043793cbf7cd48e64b626a01b457db1fb3fe))
* **deps:** bump go.opentelemetry.io/collector/pdata from 1.42.0 to 1.43.0 ([#141](https://github.com/openkcm/common-sdk/issues/141)) ([2f25d93](https://github.com/openkcm/common-sdk/commit/2f25d93041fb4ad8cd20cc65fdf8f3b1d2d67335))
* **deps:** bump golang.org/x/time from 0.13.0 to 0.14.0 ([#142](https://github.com/openkcm/common-sdk/issues/142)) ([1b5cb2f](https://github.com/openkcm/common-sdk/commit/1b5cb2f11e0b2fb2c1093d8bd46985f52a5cb9ae))
* **deps:** bump google.golang.org/grpc from 1.75.1 to 1.76.0 ([#140](https://github.com/openkcm/common-sdk/issues/140)) ([5910ed7](https://github.com/openkcm/common-sdk/commit/5910ed7c0fe1f76e26bdb597e577c88b9c8b50a8))

## [1.4.6](https://github.com/openkcm/common-sdk/compare/v1.4.5...v1.4.6) (2025-10-07)


### Bug Fixes

* add configurable config file ([#136](https://github.com/openkcm/common-sdk/issues/136)) ([04423f2](https://github.com/openkcm/common-sdk/commit/04423f265e94013d17c0ad3cad1552895c7c0b9c))
* cover on loading data for new included yaml format ([#138](https://github.com/openkcm/common-sdk/issues/138)) ([0f4f217](https://github.com/openkcm/common-sdk/commit/0f4f21724886bc52998039d6e383dfb1952773e6))

## [1.4.5](https://github.com/openkcm/common-sdk/compare/v1.4.4...v1.4.5) (2025-10-03)


### Bug Fixes

* common grpc and fs updates ([#135](https://github.com/openkcm/common-sdk/issues/135)) ([e11896a](https://github.com/openkcm/common-sdk/commit/e11896ac611495eb9735e46b1252fc9a920aa8dc))

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
