package main

// FFmpeg expects to have all its input video accessible when you start it.
// We hack around this expectation by giving it a concat list of FIFOs. FFmpeg would
// process those FIFOs as fast as it could, but since we'll be the producers of those
// FIFOs we can make them block until more data comes available.
//
// tl;dr: we'll feed it with realtime data as it becomes available.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/function61/gokit/os/osutil"
	"golang.org/x/sys/unix"
)

func ffmpegWithOnTheFlyInput(
	ctx context.Context,
	itemCount int,
	tempDir string,
	produce func(ffmpegInput io.Writer, idx int) error,
	runFfmpeg func(concatFilename string) error,
) error {
	fifo1 := filepath.Join(tempDir, "fifo1")
	fifo2 := filepath.Join(tempDir, "fifo2")

	if err := coalesce(mkfifo(fifo1), mkfifo(fifo2)); err != nil {
		return err
	}

	// ffmpeg seems to open the next file while the old is open (or there is some other
	// race condition), so we juggle between two fifos which seems to work nicely
	fifoByIndex := func(idx int) string {
		if idx%2 == 0 {
			return fifo1
		} else {
			return fifo2
		}
	}

	ffmpegInputFile := filepath.Join(tempDir, "inputfiles.txt")

	if err := func() error {
		ffmpegInputFifos := []string{}
		for idx := 0; idx < itemCount; idx++ {
			ffmpegInputFifos = append(ffmpegInputFifos, fifoByIndex(idx))
		}

		return writeFfmpegConcatInputFile(ffmpegInputFile, ffmpegInputFifos)
	}(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- runFfmpeg(ffmpegInputFile)
	}()

	feedIntoFifo := func(destination string, idx int) error {
		ffmpegInput, err := os.OpenFile(destination, os.O_WRONLY, 0660)
		if err != nil {
			return err
		}
		defer ffmpegInput.Close() // double close intentional

		// responsible for writing item to the ffmpeg
		if err := produce(ffmpegInput, idx); err != nil {
			return err
		}

		return ffmpegInput.Close()
	}

	for idx := 0; idx < itemCount; idx++ {
		if err := feedIntoFifo(fifoByIndex(idx), idx); err != nil {
			return err
		}
	}

	return <-done
}

func writeFfmpegConcatInputFile(filePath string, filenames []string) error {
	lines := []string{}
	for _, filename := range filenames {
		lines = append(lines, fmt.Sprintf("file '%s'", filename))
	}

	if len(lines) == 0 {
		return errors.New("no input images")
	}

	if err := ioutil.WriteFile(filePath, []byte(strings.Join(lines, "\n")), osutil.FileMode(osutil.OwnerRW, osutil.GroupNone, osutil.OtherNone)); err != nil {
		return err
	}

	return nil
}

func mkfifo(filePath string) error {
	/* Why doesn't this work?
	if true {
		h, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, os.ModeNamedPipe|0660)
		if err != nil {
			return err
		}
		return h.Close()
	}
	*/

	return unix.Mkfifo(filePath, 0660|unix.S_IFIFO)
}

func coalesce(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
