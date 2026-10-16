# SiriusXM LAN Sidecar

A small LAN HTTP sidecar for [STR](https://github.com/JRpersonal/streborn). It runs `sxm.py` on the network host, then
turns the HLS streams published by that script into ordinary HTTP MP3 streams
that a Bose speaker can fetch directly.

## Usage

Place your existing `sxm.py` next to the executable, then run:

```bash
./siriusxm-sidecar USERNAME PASSWORD
```

On Windows the executable will normally be `siriusxm-sidecar.exe`:

```powershell
.\siriusxm-sidecar.exe USERNAME PASSWORD
```

The server listens on `0.0.0.0:8091` by default. STR should use the LAN URL,
for example:

```text
http://192.168.1.238:8091
```

Port 8090 is intentionally avoided because the Bose device already uses it.

## Requirements

- Go 1.22+
- Python 3
- `sxm.py`
- FFmpeg with `ffmpeg` available on PATH
- An authenticated SiriusXM account for the external script

The sidecar does **not** implement a SiriusXM protocol or decryption layer of
its own. `sxm.py` is treated as an external media producer. Its command line is
currently expected to be:

```text
python sxm.py USERNAME PASSWORD
```

If the script uses different names or locations, set:

```text
SXM_PYTHON=python3
SXM_SCRIPT=/path/to/sxm.py
SXM_HLS_HOST=127.0.0.1
SXM_HLS_PORT=9998
FFMPEG=/path/to/ffmpeg
```

The sidecar expects the script to publish HLS at paths matching the station
catalog, e.g. `http://127.0.0.1:9998/101.m3u8`.

## HTTP API

### GET `/api/health`

```json
{"status":"ok"}
```

### GET `/api/stations`

Returns the station list STR can display without knowing anything about
SiriusXM:

```json
[
  {"id":"octane","name":"Octane"},
  {"id":"siriushits1","name":"SiriusXM Hits 1"}
]
```

### GET `/api/stations/{id}`

```json
{
  "id":"octane",
  "name":"Octane",
  "stream_url":"http://192.168.1.238:8091/api/stations/octane/stream",
  "content_type":"audio/mpeg",
  "title":"Artist - Song Title"
}
```

### GET `/api/stations/{id}/now-playing`

Returns title information extracted from the current HLS playlist when it is
available.

### GET `/api/stations/{id}/stream`

The sidecar starts FFmpeg on demand, reads the station's HLS URL, and streams
MP3 audio to the requesting speaker. Nothing is written to disk.

## Network behavior

The sidecar must run on a PC, NAS, Raspberry Pi, or another machine reachable
from the Bose speaker. The stream URL must use the host's LAN IP; `127.0.0.1`
is only used internally between the sidecar and `sxm.py`.

Make sure the host firewall permits TCP 8091 from the speaker's LAN.

## Build

```bash
go build -o siriusxm-sidecar ./server
```

For Windows:

```powershell
go build -o siriusxm-sidecar.exe ./server
```

## How the bridge connects to `sxm.py`

The sidecar does not embed Python. It starts your existing script as a child
process, passing the credentials exactly as two positional arguments:

```text
python sxm.py USERNAME PASSWORD
```

The existing relay setup used a local HLS publisher on port 9998 with station
paths like `<source-id>.m3u8`. The sidecar follows that layout. For example,
`octane` maps to `/9413.m3u8` in the catalog used by this project.

The sidecar then turns:

```text
sxm.py -> http://127.0.0.1:9998/<source-id>.m3u8
```

into:

```text
Bose speaker -> http://<LAN-IP>:8091/api/stations/<id>/stream
```

FFmpeg performs the HLS-to-MP3 conversion on demand. The audio is streamed
through memory/pipes; it is not written to disk.

If your `sxm.py` publishes its HLS output on another port, set `SXM_HLS_PORT`.
If it uses another path convention, the `StationCatalog` entries in
`catalog.go` are the single place to adjust the source IDs.

## Credentials

The requested command-line form is convenient for testing:

```text
./siriusxm-sidecar USERNAME PASSWORD
```

Because command-line arguments can be visible to other local processes,
consider using a protected service account or changing the launcher later to
read credentials from environment variables or a protected config file.
