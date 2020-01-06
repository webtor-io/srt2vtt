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
   0.0.1

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --host value, -H value                             listening host
   --http-port value, --Ph value                      http listening port (default: 8080)
   --access-control-allow-origin value, --acao value  Access-Control-Allow-Origin header value (default: "*") [$ACCESS_CONTROL_ALLOW_ORIGIN]
   --probe-port value, --pP value                     probe port (default: 8081)
   --help, -h                                         show help
   --version, -v                                      print the version
```
