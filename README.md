# tts-shims

A family of tiny, fast HTTP shims that each speak a **third-party TTS provider's
API** on the front and call **[Speechify](https://docs.speechify.ai)** on the
back. Point any tool that expects OpenAI, ElevenLabs, Cartesia, AWS Polly,
Deepgram, Vapi, Rime, LMNT, Hume, Fish, Google Cloud TTS, MiniMax, Inworld, or Resemble
at the matching shim and it synthesizes with Speechify instead — no client
changes.

Built for "bring your own TTS" (BYOC) integrations in voice-agent platforms such
as [Deepgram Voice Agent](https://developers.deepgram.com/docs/voice-agent-tts-models),
which send a provider-formatted request to a configurable endpoint and expect
audio back.

- **Zero dependencies** — Go standard library only.
- **One shared engine** — every provider implements a small `Provider` interface;
  a single handler wires it to Speechify.
- **Two backends** — a streaming path (raw chunked audio, lowest latency) and a
  base64 path (for providers whose native response is base64/hex in JSON).
- **One static binary per provider** — pick the one you need; distroless images.

## Providers

Each provider is a binary under `cmd/<name>` and a package under
`providers/<name>`. All accept a placeholder credential (the shim holds the
Speechify key server-side); stream providers also support pass-through auth.

### Streaming family (raw audio → Speechify `/v1/audio/stream`)

| Provider | Binary | Route | Auth header | Format control |
|----------|--------|-------|-------------|----------------|
| OpenAI | `openai` | `POST /v1/audio/speech` | `Authorization: Bearer` | `response_format` body |
| ElevenLabs | `elevenlabs` | `POST /v1/text-to-speech/{voice_id}` | `xi-api-key` | `output_format` query |
| Cartesia | `cartesia` | `POST /tts/bytes` | `X-API-Key` | `output_format` object |
| AWS Polly | `awspolly` | `POST /v1/speech` | SigV4 (not verified) | `OutputFormat`+`SampleRate` |
| Deepgram Aura | `deepgram` | `POST /v1/speak` | `Authorization: Token` | `encoding`/`container` query |
| Vapi custom voice | `vapi` | `POST /synthesize` | `X-VAPI-SECRET` (caller-to-shim), server `SPEECHIFY_API_KEY` upstream | `message.sampleRate` body |
| Rime | `rime` | `POST /v1/rime-tts` | `Authorization: Bearer` | `Accept` header |
| LMNT | `lmnt` | `POST /v1/ai/speech/bytes` | `X-API-Key` | `format` body |
| Hume | `hume` | `POST /v0/tts/file` | `X-Hume-Api-Key` | `format.type` body |
| Fish Audio | `fish` | `POST /v1/tts` | `Authorization: Bearer` | `format` body |

### Base64 family (base64/hex in JSON → Speechify `/v1/audio/speech`)

The shim calls Speechify's batch endpoint (which returns base64), then re-wraps
the audio into the provider's native JSON envelope.

| Provider | Binary | Route | Auth header | Response envelope |
|----------|--------|-------|-------------|-------------------|
| Google Cloud TTS | `googletts` | `POST /v1/text:synthesize` | Bearer (not verified) | `{"audioContent": "<base64>"}` |
| MiniMax | `minimax` | `POST /v1/t2a_v2` | Bearer (not verified) | `{"data":{"audio":"<hex>"}}` |
| Inworld | `inworld` | `POST /tts/v1/voice` | Basic (not verified) | `{"audioContent": "<base64>"}` |
| Resemble | `resemble` | `POST /synthesize` | Bearer (not verified) | `{"audio_content": "<base64>"}` |

> MiniMax returns audio as a **hex** string, so its shim transcodes Speechify's
> base64 to hex; the others pass base64 through unchanged.

## Architecture

```
provider client / Deepgram BYOC
        │  provider-native request
        ▼
   ┌──────────────┐   Provider.Translate: request → speechify.Request + Backend
   │  shim.Handler│
   └──────┬───────┘
          │ BackendStream                 │ BackendSpeech
          ▼                               ▼
   POST /v1/audio/stream            POST /v1/audio/speech
   (raw chunked audio)              (JSON with base64 audio)
          │                               │ Provider.RenderSpeech re-wraps
          ▼                               ▼
   raw bytes (+ optional             provider-native JSON
   streaming WAV header)             envelope
```

Adding a provider is one file (`providers/<name>/<name>.go` implementing
`shim.Provider`) plus a one-line `cmd/<name>/main.go`.

### Vapi note

Vapi's custom-voice webhook is its own inbound dialect, not an OpenAI-compatible
request. Point `voice.server.url` at the deployed `vapi` shim's `/synthesize`
route. Vapi sends `{"message":{"type":"voice-request","text":"...","sampleRate":24000}}`
and the shim returns raw mono 16-bit little-endian PCM as `application/octet-stream`.
Set `VAPI_SECRET` or `SHIM_VAPI_SECRET` on the shim to require Vapi's
`X-VAPI-SECRET` header before any upstream synthesis call. Set Vapi to request
`sampleRate: 24000` unless you have verified another rate in your deployment
path.

### WAV note

Speechify's streaming endpoint rejects `wav_*` (`422`) because a length-prefixed
container is incompatible with chunked synthesis. Any provider requesting WAV is
served raw PCM with a 44-byte streaming WAV header (`0xFFFFFFFF` size fields)
prepended before the first frame — a real, playable `.wav`, exactly how OpenAI's
own endpoint streams WAV.

## Quick start

```bash
export SPEECHIFY_API_KEY=sk_your_key_here
make openai            # build one binary into ./bin
./bin/openai           # or: SHIM_ADDR=:8080 go run ./cmd/openai
```

```bash
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini-tts","input":"Hello!","voice":"coral","response_format":"mp3"}' \
  --output speech.mp3
```

## Configuration

Every binary reads the same environment (see [`.env.example`](./.env.example)):

| Variable | Default | Description |
|----------|---------|-------------|
| `SPEECHIFY_API_KEY` | *(empty)* | Speechify key injected upstream. Empty = pass-through where supported. |
| `SHIM_ADDR` | `:8080` | Listen address. |
| `SPEECHIFY_BASE_URL` | `https://api.speechify.ai` | Upstream base URL. |
| `SPEECHIFY_VERSION` | *(empty)* | Optional `Speechify-Version` pin. |
| `SHIM_DEFAULT_MODEL` | `simba-english` | Fallback Speechify model. |
| `SHIM_REQUEST_TIMEOUT` | `30s` | Per-request upstream timeout. |
| `VAPI_SECRET` / `SHIM_VAPI_SECRET` | *(empty)* | Optional shared secret required on `X-VAPI-SECRET` by the `vapi` provider. |

`GET /healthz` returns `200 ok`.

## Build

```bash
make build     # all provider binaries into ./bin
make test      # go test -race
make vet
```

### Docker

```bash
docker build --build-arg PROVIDER=openai -t speechify-ai/openai-shim .
docker run --rm -p 8080:8080 -e SPEECHIFY_API_KEY=sk_your_key speechify-ai/openai-shim
```

## License

[MIT](./LICENSE)
