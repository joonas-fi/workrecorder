package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/function61/gokit/os/osutil"
)

func makeSubtitles(fps int, timestamps []time.Time, dir string) (string, error) {
	srt := newSrtSubs()
	frame, last := srt.FrameChangeCaptioner(fps)
	defer last()

	for _, ts := range timestamps {
		frame(ts.Format("15:04:05"))
	}

	last()

	subtitlesPath := filepath.Join(dir, "subtitles.srt")

	if err := ioutil.WriteFile(
		subtitlesPath,
		[]byte(srt.Serialize()),
		osutil.FileMode(osutil.OwnerRW, osutil.GroupNone, osutil.OtherNone),
	); err != nil {
		return "", err
	}

	return subtitlesPath, nil
}

type srtSubs struct {
	items []string
}

func newSrtSubs() *srtSubs {
	return &srtSubs{
		items: []string{},
	}
}

func (s *srtSubs) Push(start time.Duration, end time.Duration, caption string) {
	/*
		1
		00:00:00,498 --> 00:00:02,827
		- Here's what I love most
		about food and diet.

	*/
	item := fmt.Sprintf(
		"%d\n%s --> %s\n%s\n",
		len(s.items)+1,
		durationToSexagesimal(start),
		durationToSexagesimal(end),
		caption)

	s.items = append(s.items, item)
}

func (s *srtSubs) Serialize() string {
	return strings.Join(s.items, "\n")
}

// helper for when you have a caption for each frame, and you don't want to detect when
// the caption changes. this internally pushes captions for frame ranges only when the
// caption changes
func (s *srtSubs) FrameChangeCaptioner(frameRate int) (func(caption string), func()) {
	// helper
	frameToDuration := func(no int) time.Duration {
		return time.Second * time.Duration(no) / time.Duration(frameRate)
	}

	currentCaption := ""
	currentCaptionStartFrameNumber := 0
	currentFrameNumber := 0

	nextFrame := func(caption string) {
		if currentCaption == "" {
			currentCaption = caption
		}

		if caption != currentCaption {
			s.Push(
				frameToDuration(currentCaptionStartFrameNumber),
				frameToDuration(currentFrameNumber),
				currentCaption)

			currentCaption = caption
			currentCaptionStartFrameNumber = currentFrameNumber
		}

		currentFrameNumber++
	}

	last := func() {
		s.Push(
			frameToDuration(currentCaptionStartFrameNumber),
			frameToDuration(currentFrameNumber),
			currentCaption)
	}

	return nextFrame, last
}

func durationToSexagesimal(dur time.Duration) string {
	// TODO: this is an ugly implementation
	bareHours := int(dur.Hours())
	dur -= time.Duration(bareHours) * time.Hour

	bareMinutes := int(dur.Minutes())
	dur -= time.Duration(bareMinutes) * time.Minute

	bareSeconds := int(dur.Seconds())
	dur -= time.Duration(bareSeconds) * time.Second

	// 00:00:00,498
	return fmt.Sprintf(
		"%.2d:%.2d:%.2d,%d",
		bareHours,
		bareMinutes,
		bareSeconds,
		int(dur.Milliseconds()))
}
