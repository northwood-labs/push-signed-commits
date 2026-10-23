# CHANGELOG

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com), adheres to [Semantic Versioning](https://semver.org), and uses [Conventional Commit](https://www.conventionalcommits.org) syntax.

## Unreleased

### :books: Documentation

* [`d375ae5`](https://github.com/northwood-labs/push-signed-commits/commit/d375ae5216f1fbaa87ad99e0712a55d6941aac10): Improve `README.md` documentation and formatting. ([@skyzyx](https://github.com/skyzyx))

### <!-- 0 -->:rocket: Features

* [`ef9ee7f`](https://github.com/northwood-labs/push-signed-commits/commit/ef9ee7f61201dfcff5701d17801b50febf39b602): Initial commit ([@harshita-gupta](https://github.com/harshita-gupta))
* [`fdb8d93`](https://github.com/northwood-labs/push-signed-commits/commit/fdb8d9333ff0b4ee3f328549f0b6e077f4955202): Create a composite Github Action that creates signed copies of local commits on a remote branch ([#1](https://github.com/northwood-labs/push-signed-commits/issues/1)) ([@harshita-gupta](https://github.com/harshita-gupta))
* [`6558b29`](https://github.com/northwood-labs/push-signed-commits/commit/6558b296891749a347119486f81a7447b7ea55de): Improve error handling, add more logs on progress through the commits, update default log level to info. ([@harshita-gupta](https://github.com/harshita-gupta))
* [`a659be7`](https://github.com/northwood-labs/push-signed-commits/commit/a659be76ef3c58e190e7977ddcbeceda074728cb): Raise an error if we push no commits, instead of just printing an error and exiting with success. Also, drop log level to WARN. ([@harshita-gupta](https://github.com/harshita-gupta))
* [`b4bf811`](https://github.com/northwood-labs/push-signed-commits/commit/b4bf81149fee65dae2906b09b9a87f3178cfbfde): Add python-version input to allow callers to specify Python version ([@kan-o-ash](https://github.com/kan-o-ash))
* [`0166296`](https://github.com/northwood-labs/push-signed-commits/commit/0166296dd9a980c529b4c7e2c91cca0a244cf2d2): Add comprehensive project configuration management system. ([@skyzyx](https://github.com/skyzyx))
* [`3471ced`](https://github.com/northwood-labs/push-signed-commits/commit/3471ced671a8596ef82b714281754bc2f7c2dfc8): Implement push-signed-commits CLI tool. ([@skyzyx](https://github.com/skyzyx))

### <!-- 1 -->:bug: Bug Fixes

* [`ee87d3f`](https://github.com/northwood-labs/push-signed-commits/commit/ee87d3fa834b6232d2201d3684c63f6ce83d2048): Show full response if keys aren't what we expect. Don't fail the full github action step unless it's a partial failure. ([@harshita-gupta](https://github.com/harshita-gupta))
* [`b7268a8`](https://github.com/northwood-labs/push-signed-commits/commit/b7268a8b87ebbeea2a3dbd6f7e57cb47c05f965c): Don't assume utf-8 ([@cfal](https://github.com/cfal))
* [`7faff5f`](https://github.com/northwood-labs/push-signed-commits/commit/7faff5fffd9a6845be84a6f381ac28afbfc0935a): Support chunking for big files. ([@cfal](https://github.com/cfal))

<p>Generated on 2026-10-22.</p>
