package main

// These all were copy-pasted from xgraphics because newDrawable() didn't support user-specified geometry

import (
	"fmt"
	"image"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xrect"
	"github.com/BurntSushi/xgbutil/xwindow"
)

//nolint:unused,deadcode
func newDrawable(X *xgbutil.XUtil, did xproto.Drawable) (*xgraphics.Image, error) {
	// Get the geometry of the pixmap for use in the GetImage request.
	pgeom, err := xwindow.RawGeometry(X, xproto.Drawable(did))
	if err != nil {
		return nil, err
	}

	return newDrawableFromGeometry(X, did, pgeom)
}

func newDrawableFromGeometry(X *xgbutil.XUtil, did xproto.Drawable, pgeom xrect.Rect) (*xgraphics.Image, error) {
	// Get the image data for each pixmap.
	pixmapData, err := xproto.GetImage(X.Conn(), xproto.ImageFormatZPixmap,
		did,
		int16(pgeom.X()), int16(pgeom.Y()), uint16(pgeom.Width()), uint16(pgeom.Height()),
		(1<<32)-1).Reply()
	if err != nil {
		return nil, err
	}

	// Now create the xgraphics.Image and populate it with data from
	// pixmapData and maskData.
	ximg := xgraphics.New(X, image.Rect(0, 0, pgeom.Width(), pgeom.Height()))

	// We'll try to be a little flexible with the image format returned,
	// but not completely flexible.
	err = readDrawableData(X, ximg, did, pixmapData,
		pgeom.Width(), pgeom.Height())
	if err != nil {
		return nil, err
	}

	return ximg, nil
}

func readDrawableData(X *xgbutil.XUtil, ximg *xgraphics.Image, did xproto.Drawable,
	imgData *xproto.GetImageReply, width, height int) error {

	format := xgraphics.GetFormat(X, imgData.Depth)
	if format == nil {
		return fmt.Errorf("Could not find valid format for pixmap %d "+
			"with depth %d", did, imgData.Depth)
	}

	switch format.Depth {
	case 1: // We read bitmaps in as alpha masks.
		if format.BitsPerPixel != 1 {
			return fmt.Errorf("The image returned for pixmap id %d with "+
				"depth %d has an unsupported value for bits-per-pixel: %d",
				did, format.Depth, format.BitsPerPixel)
		}

		// Calculate the padded width of our image data.
		pad := int(X.Setup().BitmapFormatScanlinePad)
		paddedWidth := width
		if width%pad != 0 {
			paddedWidth = width + pad - (width % pad)
		}

		// Process one scanline at a time. Each 'y' represents a
		// single scanline.
		for y := 0; y < height; y++ {
			// Each scanline has length 'width' padded to
			// BitmapFormatScanlinePad, which is found in the X setup info.
			// 'i' is the index to the starting byte of the yth scanline.
			i := y * paddedWidth / 8
			for x := 0; x < width; x++ {
				b := imgData.Data[i+x/8] >> uint(x%8)
				if b&1 > 0 { // opaque
					ximg.Set(x, y, xgraphics.BGRA{
						B: 0x0,
						G: 0x0,
						R: 0x0,
						A: 0xff,
					})
				} else { // transparent
					ximg.Set(x, y, xgraphics.BGRA{
						B: 0xff,
						G: 0xff,
						R: 0xff,
						A: 0x0,
					})
				}
			}
		}
	case 24, 32:
		switch format.BitsPerPixel {
		case 24:
			bytesPer := int(format.BitsPerPixel) / 8
			var i int
			ximg.For(func(x, y int) xgraphics.BGRA {
				i = y*width*bytesPer + x*bytesPer
				return xgraphics.BGRA{
					B: imgData.Data[i],
					G: imgData.Data[i+1],
					R: imgData.Data[i+2],
					A: 0xff,
				}
			})
		case 32:
			bytesPer := int(format.BitsPerPixel) / 8
			var i int
			ximg.For(func(x, y int) xgraphics.BGRA {
				i = y*width*bytesPer + x*bytesPer
				return xgraphics.BGRA{
					B: imgData.Data[i],
					G: imgData.Data[i+1],
					R: imgData.Data[i+2],
					A: imgData.Data[i+3],
				}
			})
		default:
			return fmt.Errorf("The image returned for pixmap id %d has "+
				"an unsupported value for bits-per-pixel: %d",
				did, format.BitsPerPixel)
		}

	default:
		return fmt.Errorf("The image returned for pixmap id %d has an "+
			"unsupported value for depth: %d", did, format.Depth)
	}

	return nil
}
