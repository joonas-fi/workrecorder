package main

import (
	"image"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xrect"
)

type randrOutput struct {
	Id   randr.Output
	Info randr.GetOutputInfoReply
	Crtc randr.GetCrtcInfoReply
}

func (r *randrOutput) ScreenId() ScreenId {
	return ScreenId(r.Info.Name)
}

func (r *randrOutput) Rect() image.Rectangle {
	conf := r.Crtc // shorthand

	return image.Rect(
		int(conf.X),
		int(conf.Y),
		int(uint16(conf.X)+conf.Width),
		int(uint16(conf.Y)+conf.Height))
}

func (r *randrOutput) XRect() xrect.Rect {
	rect := r.Rect()

	width := rect.Max.X - rect.Min.X
	height := rect.Max.Y - rect.Min.Y

	return xrect.New(
		rect.Min.X,
		rect.Min.Y,
		width,
		height)
}

func getConnectedOutputs(X *xgb.Conn, root xproto.Window) ([]randrOutput, error) {
	// Gets the current screen resources. Screen resources contains a list
	// of names, crtcs, outputs and modes, among other things.
	resources, err := randr.GetScreenResources(X, root).Reply()
	if err != nil {
		return nil, err
	}

	connectedOutputs := []randrOutput{}

	for _, output := range resources.Outputs {
		outputInfo, err := randr.GetOutputInfo(X, output, 0).Reply()
		if err != nil {
			return nil, err
		}

		if outputInfo.Connection != randr.ConnectionConnected { // skip disconnected
			continue
		}

		// CRTC ("CRT Controller") is jargon for display controller.
		// outputInfo.Crtcs "is the list of CRTCs that this output may be connected to"
		// (= NOT currently connected to)
		crtc, err := randr.GetCrtcInfo(X, outputInfo.Crtc, 0).Reply()
		if err != nil {
			return nil, err
		}

		connectedOutputs = append(connectedOutputs, randrOutput{
			Id:   output,
			Info: *outputInfo,
			Crtc: *crtc,
		})
	}

	return connectedOutputs, nil
}
