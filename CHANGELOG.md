# Changelog

## [8.9.0](https://github.com/elastic/elastic-transport-go/compare/v8.8.0...v8.9.0) (2026-02-24)


### Features

* Add functional options pattern for transport client creation ([#74](https://github.com/elastic/elastic-transport-go/issues/74)) ([dbc2c13](https://github.com/elastic/elastic-transport-go/commit/dbc2c13f678b3c48548a4518844243bc62b4ae14))
* Upgrade Go from 1.20 to 1.21 ([209a7aa](https://github.com/elastic/elastic-transport-go/commit/209a7aa7b589008105f8592933143e5ae479ded4))


### Bug Fixes

* Add defensive check in discovery to prevent panic when no URLs are configured ([dbc2c13](https://github.com/elastic/elastic-transport-go/commit/dbc2c13f678b3c48548a4518844243bc62b4ae14))
* Don't put gzipWriters with errors back into the pool ([#62](https://github.com/elastic/elastic-transport-go/issues/62)) ([6b78c54](https://github.com/elastic/elastic-transport-go/commit/6b78c54216c250f4d4a54f97b64cf3f4b2a0791e))
* Ensure global HTTP request headers are set correctly in transport client ([#64](https://github.com/elastic/elastic-transport-go/issues/64)) ([bfc0323](https://github.com/elastic/elastic-transport-go/commit/bfc0323ed09332e3d0de5a26dc6fdc002bb95494))
* Improve client pool concurrency safety and performance ([#67](https://github.com/elastic/elastic-transport-go/issues/67)) ([6507084](https://github.com/elastic/elastic-transport-go/commit/65070846315ed00cf2a842b61defb56f8dc60121))
* Linting rules satisfied ([793a813](https://github.com/elastic/elastic-transport-go/commit/793a813e30672258631f0043950a8fd4f6a09cef))
* Prevent drainErrChan from missing errors due to non-blocking receive ([#65](https://github.com/elastic/elastic-transport-go/issues/65)) ([0f0e5ac](https://github.com/elastic/elastic-transport-go/commit/0f0e5ac78d63cddd77cb951e6ccbaf2388861812))
* Remove deprecated code ([#53](https://github.com/elastic/elastic-transport-go/issues/53)) ([793a813](https://github.com/elastic/elastic-transport-go/commit/793a813e30672258631f0043950a8fd4f6a09cef))
* Replace slice manipulation with slices.Delete ([209a7aa](https://github.com/elastic/elastic-transport-go/commit/209a7aa7b589008105f8592933143e5ae479ded4))
* Return pooled gzip buffers on compression errors ([#70](https://github.com/elastic/elastic-transport-go/issues/70)) ([dfdb552](https://github.com/elastic/elastic-transport-go/commit/dfdb552c5bdf0f72bb398f288a7167646c96ecd3))
* Use net.SplitHostPort in getNodeURL for IPv6 support ([#63](https://github.com/elastic/elastic-transport-go/issues/63)) ([e2d86cf](https://github.com/elastic/elastic-transport-go/commit/e2d86cffb31e7b74f2e77eb74dfab221edf8514e))


### Performance Improvements

* Reduce metrics hot-path lock contention ([#72](https://github.com/elastic/elastic-transport-go/issues/72)) ([9d402f8](https://github.com/elastic/elastic-transport-go/commit/9d402f88d66e38d1abaa014c40fb4a980babad16))
* Replace roundRobinSelector initialization with newRoundRobinSelector function for improved atomicity [#71](https://github.com/elastic/elastic-transport-go/issues/71) ([15632e3](https://github.com/elastic/elastic-transport-go/commit/15632e3a30a7ec37262c0bbee7d14c8c882950fa))

## [8.8.0](https://github.com/elastic/elastic-transport-go/compare/v8.7.0...v8.8.0) (2025-11-19)


### Features

* add a Close method to transport ([#36](https://github.com/elastic/elastic-transport-go/issues/36)) ([b2d94de](https://github.com/elastic/elastic-transport-go/commit/b2d94deb8ad1efd05eab3ee465679b7bd4e42942))
* add interceptor pattern ([#35](https://github.com/elastic/elastic-transport-go/issues/35)) ([c2d0c18](https://github.com/elastic/elastic-transport-go/commit/c2d0c18106e550ed73c30f49e2b318f51a6e57db))
