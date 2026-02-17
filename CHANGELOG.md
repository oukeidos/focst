# Changelog

All notable changes to this project will be documented in this file.

## [0.1.1] - 2026-02-16

### Changed
- Updated `README.md` installation guide to refer to the Windows installer as EXE (not MSI).
- Extended Japanese preprocessing to remove text inside fullwidth brackets `（）` and `［］` (same behavior as `()` and `[]`).
- Added VTT-specific preprocessing that merges consecutive segments with identical start/end timestamps into a single multi-line segment in source order.

## [0.1.0] - 2026-02-07

### Added
- focst initial release.
- focst-gui initial release.
