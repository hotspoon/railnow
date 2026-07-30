# Changelog

All notable changes to JadwalKRL are documented in this file. Releases follow Semantic Versioning.

## [0.2.0] - 2026-07-30

### Added

- Schedule snapshot update status with source and effective-date details.
- Mobile-friendly searches from the current time or a selected departure time.
- Live next-train cards, countdowns, refresh, retry, and removal for device-local saved routes.
- Go, Vitest, and Playwright coverage for the new behavior on Chromium and WebKit mobile viewports.

### Changed

- Departure day handling now supports today, tomorrow, and later service-day offsets.
- Search time is retained in HTMX URLs and train-detail navigation.
