---
name: audio-transcriber
description: Use when user provides an audio file path, asks to transcribe audio, listen to a recording, or process voice messages. Triggers on file extensions like .opus, .mp3, .m4a, .wav, .ogg, .flac, .webm, or mentions of WhatsApp audio, voice notes, recordings, or podcasts.
---

# Audio Transcriber

Transcribe audio files locally using OpenAI Whisper. Claude cannot hear — this skill bridges that gap by running Whisper on the user's machine.

## When to Use

- User drops an audio file path (any format: `.opus`, `.mp3`, `.m4a`, `.wav`, `.ogg`, `.flac`, `.webm`, `.aac`, `.wma`)
- User asks to "listen to", "transcribe", or "process" a recording
- User mentions WhatsApp audio, voice notes, or voice messages
- User wants subtitles/captions generated from audio or video

## Prerequisites Check

Before transcribing, verify the toolchain:

```bash
# 1. Check if Whisper is installed
which whisper

# 2. If not installed, install via pipx (preferred on macOS)
pipx install openai-whisper

# 3. If pipx unavailable, use pip with venv
python3 -m venv /tmp/whisper-env && source /tmp/whisper-env/bin/activate && pip install openai-whisper
```

## Transcription Command

```bash
whisper "<audio-file-path>" \
  --model medium \
  --output_dir "<scratchpad-dir>" \
  --output_format txt
```

**Use the session scratchpad directory for output** — never pollute the user's project.

## Model Selection

| Model | Size | Speed | Accuracy | Use When |
|-------|------|-------|----------|----------|
| `tiny` | 39MB | Fastest | Low | Quick gist, short clear audio |
| `base` | 74MB | Fast | Fair | Simple, clear speech |
| `small` | 244MB | Moderate | Good | Most casual use |
| `medium` | 1.5GB | Slow | Great | **Default — best tradeoff** |
| `large-v3` | 3GB | Slowest | Best | Non-English, heavy accents, noisy audio |

**Default to `medium`.** Upgrade to `large-v3` if user reports poor quality or audio is non-English with heavy dialect.

## Language Handling

- Whisper auto-detects language — no flag needed in most cases
- Force language if auto-detection fails: `--language de` (ISO code or full name)
- For **translation to English**, use: `--task translate`

## Output Formats

| Format | Flag | Use Case |
|--------|------|----------|
| `.txt` | `--output_format txt` | **Default** — plain transcript |
| `.srt` | `--output_format srt` | Subtitles with timestamps |
| `.vtt` | `--output_format vtt` | Web subtitles (WebVTT) |
| `.json` | `--output_format json` | Programmatic access, word-level timestamps |
| all | `--output_format all` | Generate every format |

## Workflow

1. **Verify file exists** — `ls -la "<path>"`
2. **Check Whisper installed** — install if missing
3. **Run transcription** with `medium` model, output to scratchpad
4. **Read the `.txt` output** and present to user
5. **Offer follow-ups**: summarize, translate, extract action items, analyze

## Common Issues

| Problem | Fix |
|---------|-----|
| `pip install` blocked by PEP 668 | Use `pipx install openai-whisper` instead |
| Whisper not found after install | Check `~/.local/bin` is in PATH, or use full path |
| Poor transcription quality | Upgrade to `large-v3`, or force correct `--language` |
| Unsupported format error | Convert first: `ffmpeg -i input.xyz output.wav` |
| Very long audio (>30 min) | Still works, just takes time. Warn user about duration. |

## Multiple Files

Whisper accepts multiple files:

```bash
whisper file1.opus file2.mp3 file3.m4a --model medium --output_dir "<scratchpad>"
```

Or loop for separate handling:

```bash
for f in /path/to/audios/*.opus; do
  whisper "$f" --model medium --output_dir "<scratchpad>" --output_format txt
done
```
