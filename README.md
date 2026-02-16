# FoCST: Format Constrained Subtitle Translator 

`focst` is a multilingual subtitle translation tool using Gemini models.

- It enforces Gemini output format (two lines per segment, CPL limits) for subtitle quality.
- It can retry only failed chunks in a separate session using a recovery log.
- It supports both CLI and GUI workflows.

New users: read Disclaimer, then Quick Start (GUI), then Setup, then Using the GUI.

## Disclaimer (Read First)

- The software is provided "as-is" with no warranty or liability; it is not affiliated with Google, OpenAI, or other providers.
- Third-party APIs (Gemini, OpenAI) apply their own terms, pricing, rate limits, and data policies; you are responsible for keys and costs.
- You are responsible for rights, licenses, and platform compliance for any subtitle content you translate or distribute.
- Subtitles and inputs may be sent to external APIs; local logs and recovery files may be created; keys can be stored in your OS keychain.
- Translations can be wrong or incomplete; you must review results before use or release.
- See [DISCLAIMER.md](DISCLAIMER.md) for the full text.

## Quick Start (GUI first)

1. Download and install the latest build from GitHub Releases.
2. Launch the GUI and set your Gemini API key (Keychain is recommended).
3. Drag and drop a subtitle file onto the window (or click the + icon) and check the output file.

Notes:
- If you save the key to the OS keychain, it is stored on this computer. Use caution on shared machines.
- Default translation is Japanese -> Korean. Change it in the Settings window (three-dot button).

## Installation

### Prebuilt Binaries (Recommended)

Prebuilt packages are intended to be published on GitHub Releases.

- Windows (EXE)
  - Installs per-user by default (for example, under your user profile).
  - Expect Start Menu and desktop shortcuts.
  - Windows SmartScreen may show a warning for unsigned or new binaries.
- Linux (AppImage)
  - Grant execute permission, then run the AppImage.
  - Optional: integrate with your desktop using AppImageLauncher or similar tools.
- Linux (CLI tar.gz)
  - Includes the `focst` binary plus license/docs.
  - Extract and run `./focst --version` to verify.

### Build from Source

Requirements:
- Go 1.25.6+ (see `go.mod`)
- CGO enabled for the GUI (required by Fyne)

Build commands:

```bash
# CLI
go build ./cmd/focst

# GUI (requires CGO and Fyne system dependencies)
go build ./cmd/focst-gui
```

## Setup (API Keys and Providers)

`focst` calls external APIs for translation and name extraction.

- Gemini API is required for translation.
- OpenAI API is required for the `names` feature (it uses web search).

### GUI Setup (Recommended for beginners)

- Open the GUI and enter your Gemini API key on the key screen.
- Choose either:
  - Save: stores the key in the OS keychain.
  - Once: keeps the key only for the current session.
- If you use the Dictionary/Names feature, save an OpenAI key in the Settings window (Keys tab).

Notes:
- The GUI does not read environment variables.
- Keychain storage writes to the local system; use caution on shared computers.

### CLI Setup (Advanced / Automation)

Recommended: store keys in your OS keychain:

```bash
focst env setup --service gemini
focst env setup --service openai
focst env status --service gemini
```

Key resolution order in the CLI:
1. OS keychain (default)
2. Environment variables if `--allow-env` is set
3. Interactive prompt if running in a terminal

Environment variables (optional):
- `GEMINI_API_KEY`
- `OPENAI_API_KEY`

Options:
- `--allow-env` allows reading the environment variables.
- `--env-only` ignores keychain and requires the env vars.

Costs and rate limits are your responsibility; check your provider dashboard.

## Using the GUI

### Basic Translation Flow

- Set the Gemini API key.
- Drop a subtitle file (.srt, .vtt, .ttml, .stl, .ssa, .ass) or click the + icon.
- If you drop multiple files at once, only the first is processed; drops are ignored while a job is running.
- Check the status: success, partial success, or failure.
- Default language is Japanese -> Korean; change Source/Target in the Settings window (three-dot button).

If you save the key to the OS keychain, it is stored on this computer. Use caution on shared machines.

### Repair (Retry Failed Chunks)

- If a run partially fails, a JSON recovery log is created.
- Drop the JSON file to retry only the failed chunks.
- This is a recovery feature; results depend on model stability and may still fail.

### Dictionary (Name Mapping)

- Optional feature to improve name consistency.
- Create, load, overwrite, or delete dictionaries in the GUI.
- The GUI asks for confirmation before overwriting a dictionary file.
- Name extraction uses the OpenAI key saved in the Keys tab.
- Dictionaries are stored in `~/.focst/names/`.
- On Windows, uninstalling FoCST does not delete these dictionary files.

### Advanced Tab

You can adjust:
- Chunk size, context size, and concurrency
- Retry on long lines (CPL validation)
- Prompt CPL enforcement (line length guidance in the model prompt)
- Preprocess and postprocess toggles (full or language-specific)
- Max output tokens for name extraction

### Output and Overwrite Policy

- GUI output uses the target language suffix (for example `_ko`).
- If a file already exists, a numeric or UUID suffix is added.
- The GUI does not overwrite existing output by default.

## Using the CLI

Basic example:

```bash
focst input.srt output.srt
```

### Core Commands

- `translate` (default): translate subtitles with Gemini.
- `repair`: resume failed chunks using a recovery log.
- `names`: generate a character name mapping using OpenAI (requires a separate key).
- `list`: show supported language codes.
- `env`: manage keys in your OS keychain.

### Common Options

- `--source`, `--target`: language codes (default `ja` -> `ko`). Use `focst list` to find codes.
- `--model`: Gemini model ID (default `gemini-3-flash-preview`).
- `--chunk-size`, `--context-size`, `--concurrency`: performance and context tuning.
- `--retry-on-long-line`: retry when lines exceed the CPL-based limit.
- `--no-prompt-cpl`: disable CPL constraints in the translation prompt.
- `--no-preprocess`, `--no-postprocess`: disable all preprocessing/postprocessing.
- `--no-lang-preprocess`, `--no-lang-postprocess`: disable only language-specific rules.
- `--names`: JSON mapping file for character names.
- `--log-file`: append JSONL logs to a file.

For full options, run `focst --help` or `focst <command> --help`.

### Safety Guards and Limits

- Concurrency is clamped to 1-20.
- Chunk size and context size are capped at 200 and 20.
- Name extraction max tokens are capped at 128000.
- Output overwrite is opt-in (`--yes` or `-y`), otherwise the CLI prompts.

## Session Recovery and Repair

- A recovery log is created when translation ends in partial success or failure (or cancellation).
- The log is saved next to the input file and uses this naming policy:
  - `basename_recovery.json`
  - `basename_recovery_0.json` to `_9.json`
  - `basename_recovery_<UUID>.json`
- `focst repair <session_log.json>` retries only failed chunks.
- Repair requires the log file to be in the same directory as the input file.
- Logs are written with restrictive permissions (0600). See [Security and Privacy](#security-and-privacy).

## Supported Formats and Language Behavior

Formats:
- Input file extension must be one of: `.srt`, `.vtt`, `.ttml`, `.stl`, `.ssa`, `.ass` (CLI and GUI).
- Output file extension must be one of: `.srt`, `.vtt`, `.ttml`, `.stl`, `.ssa`, `.ass`.

Language behavior:
- CPL/CPS profiles are per language and used for line length limits and timing correction.
- The translator enforces a two-line output format with per-line CPL limits.
- Preprocessing is applied only for Japanese source text.
- Postprocessing is applied for Korean, Chinese, and Japanese targets.

## Security and Privacy

- Keys can be stored in the OS keychain or kept only in memory (GUI "ONCE").
- The CLI can read env vars only when explicitly enabled.
- Logs and recovery files are written with restricted permissions (0600).
- Dictionary files are written with restricted permissions and saved under `~/.focst/names/` (0700 directory).
- Sensitive values are redacted in logs where possible.
- Subtitle contents and metadata may be sent to external APIs.

## Costs, Rate Limits, and Quotas

- API usage is billed by the providers (Gemini/OpenAI).
- Rate limits and quotas are enforced by providers and may cause retries or failures.
- The CLI prints token usage and an estimated cost; this is based on embedded pricing and may be out of date.

## Troubleshooting / FAQ

- I cannot translate: confirm a valid Gemini API key exists in keychain or set `--allow-env`/`--env-only`.
- `names` fails: it requires an OpenAI key and uses web search; check quota and rate limits.
- The model is slow or unstable: try again or reduce concurrency.
- Large subtitles are slow: all segments are loaded into memory; split large files if needed.
- `--log-file` keeps growing: it appends; use a new path per run or rotate logs externally.
- "Refusing to write to a symlink path": for security, output/log paths cannot be symlinks; use a real directory/file path.
- "Existing output could not be reused": repair stops when the partial output can't be parsed or its segment count doesn't match; use `--force-repair` to re-translate without reusing the existing output (useful for automation where you prefer completion over reuse).
- "Non-interactive stdin: use --yes/-y to overwrite existing output": the CLI won't prompt without a TTY; pass `--yes` (or `-y`) or choose a new output path.
- "Model not found or no access": change the selected model in Settings or check for a newer release if a model was deprecated.
- "Lines are extremely long or awkward": disable prompt CPL enforcement (Advanced tab) or use `--no-prompt-cpl` to relax line-length guidance.

## Development

Project structure:
- `cmd/focst`: CLI entrypoint
- `cmd/focst-gui`: GUI entrypoint
- `internal/`: pipeline, translation, recovery, language profiles

Build and test:

```bash
# Run all tests
go test ./...

# Debug GUI build (enables debug state triggers)
go build -tags debug ./cmd/focst-gui
```

The GUI requires CGO and Fyne system dependencies for your OS.

## Licenses and Third-Party Notices

- Project license: MIT (`LICENSE`).
- Third-party notices: `THIRD_PARTY_NOTICES.md`.
- Full third-party license texts: `third_party_licenses/`.
- The CLI also provides `focst licenses` and `focst disclaimer`.
