# Changelog

## [1.6.0](https://github.com/openkcm/common-sdk/compare/v2.0.1...v1.6.0) (2025-11-10)


### ⚠ BREAKING CHANGES

* **auth:** redesign client data struct ([#158](https://github.com/openkcm/common-sdk/issues/158))

### Features

* add audit log events cmkAvailable and cmkUnavailable ([#147](https://github.com/openkcm/common-sdk/issues/147)) ([a56f745](https://github.com/openkcm/common-sdk/commit/a56f7452fcaf0737596c57f32bbf8e586081e975))
* add filesystem watcher and notify which file was modified over the handlers ([#121](https://github.com/openkcm/common-sdk/issues/121)) ([39b341e](https://github.com/openkcm/common-sdk/commit/39b341e8baa5c1cda0df9652ca7c88e03f183ce3))
* add issuer to clientdata ([#118](https://github.com/openkcm/common-sdk/issues/118)) ([510ca0c](https://github.com/openkcm/common-sdk/commit/510ca0cdfec22ae71ae27f959a9ed438fd5e70e8))
* Add log level trace ([#106](https://github.com/openkcm/common-sdk/issues/106)) ([e24c3a4](https://github.com/openkcm/common-sdk/commit/e24c3a47d785573d37dda5cdc138f7d3c58acbf4))
* Add new audit events and refactor ([#30](https://github.com/openkcm/common-sdk/issues/30)) ([276a262](https://github.com/openkcm/common-sdk/commit/276a262e9c9fa252f277ce7a390d1921bfaecd39))
* add new set of github actions ([#66](https://github.com/openkcm/common-sdk/issues/66)) ([0e7fc7e](https://github.com/openkcm/common-sdk/commit/0e7fc7e2d9e14928668b95a3ed067242ab7aec9e))
* add new type of credentials oauth2 based on client id and clien… ([#76](https://github.com/openkcm/common-sdk/issues/76)) ([280d260](https://github.com/openkcm/common-sdk/commit/280d26008c571dee968e60d32769f42d7893b609))
* add support for basic authentication on the SecretRef struct ([#57](https://github.com/openkcm/common-sdk/issues/57)) ([3bec4b2](https://github.com/openkcm/common-sdk/commit/3bec4b21bc07a85c64283cbeeca931b6f0c0c173))
* **audit:** unmarshal additional properties in NewLogger ([#61](https://github.com/openkcm/common-sdk/issues/61)) ([dd5636b](https://github.com/openkcm/common-sdk/commit/dd5636bb304d69fde02ff59a8463576b4334eb69))
* **auth:** add `groups` field to the client data ([#49](https://github.com/openkcm/common-sdk/issues/49)) ([7e5db43](https://github.com/openkcm/common-sdk/commit/7e5db4364853807c6a9b13345c739b986360b047))
* **auth:** Add KeyID ([9fd3489](https://github.com/openkcm/common-sdk/commit/9fd3489dfd18d8a5b6c30bad2480916d675661fb))
* **auth:** redesign client data struct ([#158](https://github.com/openkcm/common-sdk/issues/158)) ([f95d2a6](https://github.com/openkcm/common-sdk/commit/f95d2a6866462ad3750975b0471558ddfe769703))
* **ci:** add CI workflow ([9ee9584](https://github.com/openkcm/common-sdk/commit/9ee9584c12082086d1b2c1fd0472b7d615281913))
* combine load mtls config ([#90](https://github.com/openkcm/common-sdk/issues/90)) ([f9cf635](https://github.com/openkcm/common-sdk/commit/f9cf6355e2157deeccb898dc955afa79569b171f))
* define raw claims in client data ([#156](https://github.com/openkcm/common-sdk/issues/156)) ([48c91f8](https://github.com/openkcm/common-sdk/commit/48c91f8fe6901aa810bd53b7090e98bffbca03ab))
* general HTTP client support ([#148](https://github.com/openkcm/common-sdk/issues/148)) ([12349ad](https://github.com/openkcm/common-sdk/commit/12349ad087ba4661b4e401d87c99158bdb79ed38))
* include the build mechanism ([#5](https://github.com/openkcm/common-sdk/issues/5)) ([e171ba6](https://github.com/openkcm/common-sdk/commit/e171ba6ba5b5c32bcfe809c9b5ff702ccd45bda7))
* include the default tags into the common-sdk logger ([#59](https://github.com/openkcm/common-sdk/issues/59)) ([8e2fff2](https://github.com/openkcm/common-sdk/commit/8e2fff29dd618e1e32e6b5b035269d3e2a051c86))
* move the github actions into build repo ([8c27d76](https://github.com/openkcm/common-sdk/commit/8c27d763f4cefe82b59c715b4f5563d8b0032ad1))
* optimise the github workflows ([#42](https://github.com/openkcm/common-sdk/issues/42)) ([bbe9be3](https://github.com/openkcm/common-sdk/commit/bbe9be3c1aa55193a40a0a181727c1f651bdc975))
* update the github action to sign the commits of the pull request ([#40](https://github.com/openkcm/common-sdk/issues/40)) ([0d243c5](https://github.com/openkcm/common-sdk/commit/0d243c58bca1e3937c05d58b2a34b180b0dc0fb6))


### Bug Fixes

* add back the loading defaults from fields tags ([#151](https://github.com/openkcm/common-sdk/issues/151)) ([8065e68](https://github.com/openkcm/common-sdk/commit/8065e68632ad2e23bda0e579f13cbc83ca37040c))
* add configurable config file ([#136](https://github.com/openkcm/common-sdk/issues/136)) ([04423f2](https://github.com/openkcm/common-sdk/commit/04423f265e94013d17c0ad3cad1552895c7c0b9c))
* add documentation for common fs and storage ([#128](https://github.com/openkcm/common-sdk/issues/128)) ([bd9d329](https://github.com/openkcm/common-sdk/commit/bd9d32937a7b705d8c7f79113d7ba9b87b5b4d83))
* add the oauth2 base url for fetching the tokens ([#81](https://github.com/openkcm/common-sdk/issues/81)) ([0a139be](https://github.com/openkcm/common-sdk/commit/0a139be660ba60995cde72b80a2a0ba278e80575))
* adjust the content of the dependabot.yaml ([#6](https://github.com/openkcm/common-sdk/issues/6)) ([6f8b52d](https://github.com/openkcm/common-sdk/commit/6f8b52df546c85d01ac00bb0828e9411c2bd6492))
* adjust the go.mod dependencies ([#14](https://github.com/openkcm/common-sdk/issues/14)) ([8e5c5ca](https://github.com/openkcm/common-sdk/commit/8e5c5ca1fe6f3b4ca8dd70d1a80c84562a9fee0a))
* adjusts the lints ([#77](https://github.com/openkcm/common-sdk/issues/77)) ([0a0906b](https://github.com/openkcm/common-sdk/commit/0a0906bc306ee1a7474719eec915aa625544f7f5))
* assigning nil to chan that is still in use ([#143](https://github.com/openkcm/common-sdk/issues/143)) ([1db0043](https://github.com/openkcm/common-sdk/commit/1db0043793cbf7cd48e64b626a01b457db1fb3fe))
* **audit:** replace cmkid with systemid in cmkswitch ([#54](https://github.com/openkcm/common-sdk/issues/54)) ([0a33feb](https://github.com/openkcm/common-sdk/commit/0a33febc269c0d420587ca50ea7b65f7c0626efb))
* build information is passed as base64(&lt;encoded value&gt;) ([#120](https://github.com/openkcm/common-sdk/issues/120)) ([e905671](https://github.com/openkcm/common-sdk/commit/e905671f12e1f7ffae0fffb34e82d9fafdb6f84b))
* change the value to reference for couple functions ([#88](https://github.com/openkcm/common-sdk/issues/88)) ([73cd706](https://github.com/openkcm/common-sdk/commit/73cd706bbaaf6e8569e937b0a90f8e26ff7064f1))
* ci workflow GITHUB_TOKEN should be taken from secrets ([71be358](https://github.com/openkcm/common-sdk/commit/71be358a473ccd283d18926b8bd8c045ec64a075))
* common grpc and fs updates ([#135](https://github.com/openkcm/common-sdk/issues/135)) ([e11896a](https://github.com/openkcm/common-sdk/commit/e11896ac611495eb9735e46b1252fc9a920aa8dc))
* commonfs fixes and introduced a notifier ([#132](https://github.com/openkcm/common-sdk/issues/132)) ([c81ddea](https://github.com/openkcm/common-sdk/commit/c81ddea4e09c5239f14d1667573e6b00b6df49e1))
* cover on loading data for new included yaml format ([#138](https://github.com/openkcm/common-sdk/issues/138)) ([0f4f217](https://github.com/openkcm/common-sdk/commit/0f4f21724886bc52998039d6e383dfb1952773e6))
* **deps:** bump go.opentelemetry.io/collector/pdata from 1.42.0 to 1.43.0 ([#141](https://github.com/openkcm/common-sdk/issues/141)) ([2f25d93](https://github.com/openkcm/common-sdk/commit/2f25d93041fb4ad8cd20cc65fdf8f3b1d2d67335))
* **deps:** bump go.opentelemetry.io/collector/pdata from 1.43.0 to 1.44.0 ([#146](https://github.com/openkcm/common-sdk/issues/146)) ([5fa3bc3](https://github.com/openkcm/common-sdk/commit/5fa3bc309d214828937e863e5facf5be26de7228))
* **deps:** bump golang.org/x/time from 0.13.0 to 0.14.0 ([#142](https://github.com/openkcm/common-sdk/issues/142)) ([1b5cb2f](https://github.com/openkcm/common-sdk/commit/1b5cb2f11e0b2fb2c1093d8bd46985f52a5cb9ae))
* **deps:** bump google.golang.org/grpc from 1.75.1 to 1.76.0 ([#140](https://github.com/openkcm/common-sdk/issues/140)) ([5910ed7](https://github.com/openkcm/common-sdk/commit/5910ed7c0fe1f76e26bdb597e577c88b9c8b50a8))
* GITHUB_TOKEN should be taken from secrets ([c7ad324](https://github.com/openkcm/common-sdk/commit/c7ad32441a4c27181ecd352871fb685c977f862e))
* import paths for v2 ([#160](https://github.com/openkcm/common-sdk/issues/160)) ([2affe3e](https://github.com/openkcm/common-sdk/commit/2affe3e64abbade6e791ff5c5448b51ce3aca968))
* include the mtls into the oauth2 secret type ([#79](https://github.com/openkcm/common-sdk/issues/79)) ([a88d9f8](https://github.com/openkcm/common-sdk/commit/a88d9f8f41beb3995e897f753d45ed6cc690b0b9))
* load environment value from env and value fields ([#67](https://github.com/openkcm/common-sdk/issues/67)) ([90ede9d](https://github.com/openkcm/common-sdk/commit/90ede9d2bc93f8b35c3ec7356a7bd4a707e70e61))
* make generic the key value memory storage ([#126](https://github.com/openkcm/common-sdk/issues/126)) ([6679b97](https://github.com/openkcm/common-sdk/commit/6679b97afa097b8a9d7983fa6d08799c29f5ffa8))
* move the feature gates field at level of application ([#70](https://github.com/openkcm/common-sdk/issues/70)) ([ad36c84](https://github.com/openkcm/common-sdk/commit/ad36c847e1c998113cd948806d574eb30d1ea4c7))
* prevent the bump in major versions ([#163](https://github.com/openkcm/common-sdk/issues/163)) ([056b400](https://github.com/openkcm/common-sdk/commit/056b40012c1f6f215868b89e23aa08317fae9a02))
* race conditions ([#153](https://github.com/openkcm/common-sdk/issues/153)) ([2c08833](https://github.com/openkcm/common-sdk/commit/2c08833931e9f2383034b95e6b1dfe5c34f214b9))
* refactor the common file system watcher ([#124](https://github.com/openkcm/common-sdk/issues/124)) ([8d1dbf4](https://github.com/openkcm/common-sdk/commit/8d1dbf4a547ec07a98f8e0d0d905b3827aab57aa))
* refactor the oauth2 credentials and add the pointers set of func… ([#85](https://github.com/openkcm/common-sdk/issues/85)) ([4ff75d0](https://github.com/openkcm/common-sdk/commit/4ff75d0f0b36d0269cd52e69b8b6b6c04702494b))
* remove git submodules ([#31](https://github.com/openkcm/common-sdk/issues/31)) ([d2199ce](https://github.com/openkcm/common-sdk/commit/d2199ce33a030d072f46caa04a304cd072f8dc69))
* remove grouping for service environment and name ([#101](https://github.com/openkcm/common-sdk/issues/101)) ([d224657](https://github.com/openkcm/common-sdk/commit/d22465758151309e72500f7d2ed740fae1d186eb))
* Remove telemetry url config for gRPC exporters ([#149](https://github.com/openkcm/common-sdk/issues/149)) ([9ff68dd](https://github.com/openkcm/common-sdk/commit/9ff68ddbd68500f84eea3daa5ff18ef1db6d35db))
* set next dev version ([#24](https://github.com/openkcm/common-sdk/issues/24)) ([c2eb7d2](https://github.com/openkcm/common-sdk/commit/c2eb7d2c06228af417a6bd6a8ec014d62e004d68))
* stop error on created status ([3f523cd](https://github.com/openkcm/common-sdk/commit/3f523cdba6db1ec59fd6dc6b094e211da6d1821b))
* stop error on created status ([#74](https://github.com/openkcm/common-sdk/issues/74)) ([3f523cd](https://github.com/openkcm/common-sdk/commit/3f523cdba6db1ec59fd6dc6b094e211da6d1821b))
* switch defaults library ([#154](https://github.com/openkcm/common-sdk/issues/154)) ([4b9dd38](https://github.com/openkcm/common-sdk/commit/4b9dd3856c715260189d511e8182c406fcd16913))


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
