# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [staging]
### Added
- Added rendering of Markdown, and code block in to the "/sessions" page of the admin UI


## [0.2.0] - 2026-02-24 (53.5%)
### Added
- Version file and version number displayed in admin UI and Docker output
- Discord typing indicator when waiting for AI response
- Auto-discovery of MCP binary (no longer requires manual config)
- Daily memory files are now group readable/writable; context auto-refreshes when new memories arrive
- First blog post and image zoom plugin for docs site
### Changed
- Providers (Discord, Slack, Telegram) moved into `internal/providers/`, excluded from coverage
- OpenCode engine restructured to start as root and drop to unprivileged `openpact-ai` user
- Admin UI session view overhauled and theme aligned with YummyAdmin
- `/context` slash command updated to unwrap information correctly
- Homepage banner updated from "Coming Soon" to "Beta — open for testing"
- GitHub links now point directly to the public repository
### Fixed
- Security hardening: environment variables discarded after use, basic tools disabled for AI
- Docker folder permissions for volume mounts
- Docker build permissions
- Increased timeout for long-running AI tasks


## [0.1.0] - 2026-02-23 (42%)
### Added
- Initial Build

# Notes
[Deployment] Notes for deployment
[Added] for new features.
[Changed] for changes in existing functionality.
[Deprecated] for once-stable features removed in upcoming releases.
[Removed] for deprecated features removed in this release.
[Fixed] for any bug fixes.
[Security] to invite users to upgrade in case of vulnerabilities.
[YANKED] Note the emphasis, used for Hotfixes
