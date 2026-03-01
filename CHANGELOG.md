# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [staging]
### Added
- Added cron-based job scheduling system. Supports two job types: "script" (runs a Starlark script) and "agent" (starts a new AI session with a prompt). Jobs are managed via MCP tools (`schedule_list`, `schedule_create`, `schedule_update`, `schedule_delete`, `schedule_enable`, `schedule_disable`), admin API endpoints (`/api/schedules`), and a new Schedules page in the admin UI. Jobs can optionally send output to a chat channel. Persists to `secure/data/schedules.json`.
- Added `run_once` option for schedules — one-off jobs that auto-disable after execution. Supported across the store, scheduler, MCP tools, admin UI, and API.
- Added rendering of Markdown, and code block in to the "/sessions" page of the admin UI
- Added MCP tools so the AI can help the user switch the default model used. It list all available models, and will switch it for them when requested.
- Added a settings page in the Admin area to switch the default model used in new sessions.
- Added the ability to additionally fetch tools as part of the streaming messages in the admin UI. Tools by default aren't streamed, and only appear in the history once a page is refreshed.
- Added full streaming of text and tools with updates from the AI into the session pages.
- Added Discord detail mode slash commands (`/mode-simple`, `/mode-thinking`, `/mode-tools`, `/mode-full`) to control the level of detail shown in Discord responses. Thinking blocks appear as purple embeds, tool calls as orange embeds. Mode is persisted per-channel to `channel_modes.json`.
- Added admin API endpoints (`GET/PUT /api/providers/:name/mode`) for remote control of per-channel detail modes.
### Changed
- Updated the MCP server from a local standalone server triggered by OpenCode to an endpoint in the orchestrator, and passed it as a remote MCP server with auth token to OpenCode.
### Fixed
- Thinking/reasoning blocks (and tool/file/snapshot blocks) not displayed when loading historical messages on the sessions page. The Go `MessagePart` struct was dropping all fields except `type` and `text` during deserialization — replaced with `json.RawMessage` to pass OpenCode API responses through unmodified.
- Invalid JSON scheme was being passed for tools. Gemini ignored it, but Claude was stricter.
- Added full `tool` and MCP usage and information into the admin UI session logs. 
- Fixed code block width in the admin UI to prevent overflow and horizontal scrolling.


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
