package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/function61/gokit/app/dynversion"
	"github.com/function61/gokit/log/logex"
	"github.com/function61/gokit/os/osutil"
	"github.com/function61/gokit/os/systemdinstaller"
	"github.com/function61/gokit/sync/taskrunner"
	"github.com/spf13/cobra"
	"golang.org/x/image/bmp"
)

func main() {
	app := &cobra.Command{
		Use:     os.Args[0],
		Short:   "Work recorder",
		Version: dynversion.Version,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			rootLogger := logex.StandardLogger()

			osutil.ExitIfError(logic(
				osutil.CancelOnInterruptOrTerminate(rootLogger),
				rootLogger))
		},
	}

	app.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Installs as a system service",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			osutil.ExitIfError(func() error {
				service := systemdinstaller.UserService(
					"workrecorder",
					"Work recorder",
					systemdinstaller.Docs("https://github.com/joonas-fi/workrecorder"))

				if err := systemdinstaller.Install(service); err != nil {
					return err
				}

				_, err := fmt.Fprintln(os.Stderr, systemdinstaller.EnableAndStartCommandHints(service))
				return err
			}())
		},
	})

	osutil.ExitIfError(app.Execute())
}

func logic(ctx context.Context, logger *log.Logger) error {
	renderer, err := func() (string, error) {
		renderersDir := "/dev/dri"

		renderers, err := os.ReadDir(renderersDir)
		if err != nil {
			return "", err
		}

		if len(renderers) != 1 {
			return "", fmt.Errorf("expected only one renderer in %s", renderersDir)
		}

		return filepath.Join(renderersDir, renderers[0].Name()), nil
	}()
	if err != nil {
		return err
	}

	xutil, connectedOutputs, err := connectX11AndGetConnectedOutputs()
	if err != nil {
		return err
	}

	tasks := taskrunner.New(ctx, logger)

	for _, connectedOutput := range connectedOutputs {
		connectedOutput := connectedOutput // pin

		tasks.Start(string(connectedOutput.ScreenId()), func(ctx context.Context) error {
			return recordOneScreenContinuously(
				ctx,
				connectedOutput,
				renderer,
				xutil,
				logex.Levels(logex.Prefix(string(connectedOutput.ScreenId()), logger)))
		})
	}

	return tasks.Wait()
}

func recordOneScreenContinuously(
	ctx context.Context,
	connectedOutput randrOutput,
	renderer string,
	xutil *xgbutil.XUtil,
	logl *logex.Leveled,
) error {
	for {
		nextTick, err := recordOneScreen(ctx, connectedOutput, renderer, xutil, logl)
		if err != nil {
			return err
		}

		/* if we make one one minute videos with 15 seconds between frames, it's 4 frames/minute at:

		0 seconds
		15 seconds
		30 seconds
		45 seconds

		we're done at 45 second mark, so need sleep w/ nextTick amount (refers to next minute's 0 seconds)
		*/
		time.Sleep(time.Until(nextTick))
	}
}

// returns next tick
func recordOneScreen(
	ctx context.Context,
	connectedOutput randrOutput,
	renderer string,
	xutil *xgbutil.XUtil,
	logl *logex.Leveled,
) (time.Time, error) {
	logl.Info.Println("starting next video interval")

	root := xproto.Setup(xutil.Conn()).DefaultScreen(xutil.Conn()).Root

	// snap screenshot every 5 seconds and make 15-minute videos.
	// when we start this we might not be at exactly 12:15:00 though, so we start from the next
	// even 5-second mark that is in the future
	interval := 5 * time.Second
	ticks := ticksBetween(time.Now().UTC(), 15, interval)
	if len(ticks) == 0 { // can happen when we're close to the end window
		return time.Time{}, nil
	}

	nextTick := ticks[len(ticks)-1].Add(interval)

	fps := 2

	// using SHM to reduce I/O
	tempDir, err := ioutil.TempDir("/dev/shm", "workrecorder-*")
	if err != nil {
		return nextTick, err
	}

	defer os.RemoveAll(tempDir)

	subtitlesPath, err := makeSubtitles(fps, ticks, tempDir)
	if err != nil {
		return nextTick, err
	}

	videoOutputFile := filepath.Join(
		connectedOutput.ScreenId().ReadyPath(""),
		ticks[0].Format("2006-01-02"),
		fmt.Sprintf("%s.mkv", ticks[0].Format("15-04-05")))

	if err := os.MkdirAll(filepath.Dir(videoOutputFile), 0770); err != nil {
		return nextTick, err
	}

	videoOutputInMemFile := filepath.Join(tempDir, "capture.mkv")

	if err := ffmpegWithOnTheFlyInput(ctx, len(ticks), tempDir, func(ffmpegItem io.Writer, idx int) error {
		timestamp := ticks[idx]

		// wait for the wall clock to reach the timestamp
		time.Sleep(time.Until(timestamp))

		// logl.Debug.Println("frame")

		// with multi-monitor setup X's root window spans multiple monitors, therefore we ask a specific
		// rectangle inside it (whose location is specified by RANDR)
		screenshotForScreen, err := newDrawableFromGeometry(xutil, xproto.Drawable(root), connectedOutput.XRect())
		if err != nil {
			return err
		}

		// PNG uses quite a lot of CPU (it would have to get decoded back anyway), so pass it as BMP
		// without compression
		return bmp.Encode(ffmpegItem, screenshotForScreen)
	}, func(concatFilename string) error {
		ffmpeg := exec.CommandContext(
			ctx,
			"ffmpeg",
			"-hide_banner",
			"-loglevel", "error", // be less verbose
			"-vaapi_device", renderer, // looks like "/dev/dri/renderD129"
			"-r", fmt.Sprintf("%d/1", fps),
			"-f", "concat",
			"-safe", "0", // needed for file list with absolute paths
			"-i", concatFilename,
			"-i", subtitlesPath,
			"-vf", "format=nv12,hwupload,scale_vaapi=",
			"-c:v", "hevc_vaapi",
			"-qp", "24",
			videoOutputInMemFile,
		)

		ffmpeg.Stdout = os.Stdout
		ffmpeg.Stderr = os.Stderr

		return ffmpeg.Run()
	}); err != nil {
		return nextTick, err
	}

	if err := osutil.MoveFile(videoOutputInMemFile, videoOutputFile); err != nil {
		return nextTick, err
	}

	return nextTick, nil
}

type ScreenId string

func (s ScreenId) ReadyPath(additional string) string {
	return filepath.Join("/output", string(s), additional)
}

func connectX11AndGetConnectedOutputs() (*xgbutil.XUtil, []randrOutput, error) {
	xutil, err := xgbutil.NewConn()
	if err != nil {
		return nil, nil, err
	}

	X := xutil.Conn()

	if err := randr.Init(X); err != nil {
		return nil, nil, err
	}

	connectedOutputs, err := getConnectedOutputs(X, xproto.Setup(X).DefaultScreen(X).Root)
	if err != nil {
		return nil, nil, err
	}

	return xutil, connectedOutputs, nil
}
