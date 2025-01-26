# srt2vtt
Converts srt-subtitles to vtt

## Features
* Converts subtitles on the fly
* Automatically detects input encoding and converts to UTF-8

## Usage

```
% ./srt2vtt help
NAME:
   srt2vtt - converts srt to vtt

USAGE:
   srt2vtt [global options] command [command options] [arguments...]

VERSION:
   1.0.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --probe-host value  probe listening host [$PROBE_HOST]
   --probe-port value  probe listening port (default: 8081) [$PROBE_PORT]
   --use-probe         enable probe [$USE_PROBE]
   --host value        listening host [$WEB_HOST]
   --port value        http listening port (default: 8080) [$WEB_PORT]
   --help, -h          show help
   --version, -v       print the version
```

## Example

```
curl -H 'X-Source-Url: https://github.com/webtor-io/srt2vtt/raw/refs/heads/master/samples/greek.srt' 'http://localhost:8080'
```
