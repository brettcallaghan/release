**Faster resolution for large monorepos**

Resolution now caches per package. Upgrade when convenient.

### Breaking changes

- **cli:** `--target` is required ([#412](https://github.com/o/r/pull/412))

  Migration: pass `--target main` where you relied on the default.

### Fixes

- Tags with a `v` prefix are recognised ([#87](https://github.com/o/r/pull/87))
