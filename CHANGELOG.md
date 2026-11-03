# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### Fixed

- Propagate errors reported in Filer mutation responses for bucket, FilerConf,
  and IAM operations. Recursive bucket deletion no longer ignores child-entry
  failures, while not-found deletion remains idempotent.
  ([#19](https://github.com/seaweedfs/seaweedfs-cosi-driver/issues/19),
  [#20](https://github.com/seaweedfs/seaweedfs-cosi-driver/pull/20))
