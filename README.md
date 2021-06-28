![Build status](https://github.com/joonas-fi/workrecorder/workflows/Build/badge.svg)
[![Download](https://img.shields.io/docker/pulls/joonas/workrecorder.svg?style=for-the-badge)](https://hub.docker.com/r/joonas/workrecorder/)

Record computer screen 24/7 with as little resource usage (CPU, GPU & storage) as possible.


Why?
----

Sometimes it's beneficial to go "back in time" to see what was on your screen. Like:

- Accidentally deleting some code that you didn't commit yet (and was not backed up yet)
- Forgetting the name of a website that you had open
- Having the ability to retroactively prove that you worked on some tech at a specific point in time
  (let's say a patent dispute). My idea is to push my computer screen's feed encrypted in an archive
  and publish daily cryptographic digests to some kind of public ledger. That means people can't see
  what was on my screen, but I can cryptographically prove so should I want to disclose something.

Note: you're an asshole if you use this tech on somebody else's computer without their knowledge
("stalkerware") or for your employee's computer (unless they actually **want** to use this).


How to run
----------

```console
$ docker run --rm -it \
  -e "DISPLAY=unix$DISPLAY" \
  -v /tmp/.X11-unix:/tmp/.X11-unix \
  -v /home/MYUSER/workrecorder:/output \
  --user 1000:1000 \
  --group-add 109 \
  --device /dev/dri/renderD129 \
  --shm-size=512M \
  joonas/workrecorder
```

Notes:

- `/home/MYUSER/workrecorder` is the directory in which you want your videos to be saved.
  Make sure it's owned by `1000:1000`.
- `109` is the ID of group `render` (check your `/etc/group`).
- Workrecorder doesn't use that much SHM, but it's cranked up just in case you have lots of screens
  or there happens to be a rare case with much changes on screen (= larger video size)



Hardware acceleration
---------------------

Only tested with an AMD GPU (`VA-API` + `radeonsi`).
See [Arch's great documentation](https://wiki.archlinux.org/title/Hardware_video_acceleration).

Workrecorder uses the only device present under `/dev/dri` (it errors if multiple ones are present).
Workrecorder is designed to only run in a container, so having only one renderer is the case when
you map a `--device` with Docker run.


Optimizations
-------------

The easiest naïve solution would have been to:

- Snap PNG images every 5 seconds to a temporary directory
- Every 5 or 15 minutes take those images and make them into a video

However, we have the following optimizations:

- Our frame rate is only one per 5 seconds.
- Use BMP images so we don't have PNG compression/decompression overhead.
  The CPU savings are substantial.
- Saving many BMP frames on disk (or RAM) takes lots of space, so we're streaming the images to
  FFMPEG with a clever trick.
- The streaming trick has an added benefit: we're doing small work constantly instead of doing lots
  of work every 15 minutes.
- The encoded video is not saved to disk "drip-by-drip" but to RAM as a whole, before it's written
  to a disk (as not to cause unnecessary I/O). The HEVC-encoded 15-minute videos are small enough to be held in RAM
- Using modern HEVC means daily videos don't take much space.
