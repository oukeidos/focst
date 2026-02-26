# Changelog

All notable changes to this project will be documented in this file.

## [0.1.4] - 2026-02-26

### Changed
- Removed `gemini-3-pro-preview` support from embedded Gemini model metadata.
- Added GUI config fallback: if a saved model is no longer supported, it is reset to `gemini-3-flash-preview`.

## [0.1.3] - 2026-02-20

### Changed
- Removed the example text from the names extractor prompt.
- Added `gemini-3.1-pro-preview` to the embedded Gemini model metadata (GUI model list and CLI cost lookup).

## [0.1.2] - 2026-02-18

### Changed
- Added a system prompt rule: `Do NOT use "/" as a line-break substitute in subtitle text.`

## [0.1.1] - 2026-02-17

### Changed
- Added VTT-specific preprocessing that merges consecutive segments with identical start/end timestamps into a single multi-line segment in source order.
- Extended Japanese preprocessing to remove text inside fullwidth brackets `（）` and `［］` (same behavior as `()` and `[]`).
- Updated `README.md` installation guide to refer to the Windows installer as EXE (not MSI).

## [0.1.0] - 2026-02-07

### Added
- focst initial release.
- focst-gui initial release.
