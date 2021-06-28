# would love to use Alpine, but va-api doesn't seem to have AMD drivers (all I see is libva-intel-driver)
FROM ubuntu:latest

RUN apt update && apt install --no-install-recommends -y ffmpeg va-driver-all

ENTRYPOINT ["/usr/bin/workrecorder"]

ADD rel/workrecorder_linux-amd64 /usr/bin/workrecorder
