package builder

import (
	"image/color"
	"math"
	"strconv"

	"github.com/jkroepke/go-msi/internal/artwork"
	"github.com/jkroepke/go-msi/internal/rtf"
	"github.com/jkroepke/go-msi/pkg/config"
)

const (
	dlgLicense = "LicenseAgreementDlg"
	ctlAccept  = "LicenseAcceptedCheckBox"
	ctlLicense = "LicenseText"

	// licenseTop and licenseHeight place the license text between the
	// banner and the check box.
	licenseTop    = 55
	licenseHeight = 140
	acceptY       = 202
	acceptH       = 18

	// aspectTolerance is how far a theme picture's width-to-height ratio
	// may differ from its control's before the stretch distorts it.
	aspectTolerance = 0.05
)

// licenseDialog adds the license page: the license as RTF, an accept check
// box and Print. Next stays disabled until the box is checked.
func (w *wizard) licenseDialog(license string) {
	page := w.add(&uiDialog{name: dlgLicense, attrs: dlgPage, first: ctlLicense})
	page.innerHeader("End-User License Agreement", "Please read the following license agreement carefully.")
	page.control(&uiControl{
		name: ctlLicense, typ: typScrollText, text: license, ring: true,
		x: margin, y: licenseTop, w: dlgWidth - 2*margin, h: licenseHeight, attrs: ctlActive | ctlSunken,
	})
	page.control(&uiControl{
		name: ctlAccept, typ: typCheckBox, property: propLicense, text: "I &accept the terms in the License Agreement", ring: true,
		x: margin, y: acceptY, w: dlgWidth - 2*margin, h: acceptH, attrs: ctlActive, // ctlText's NoPrefix bit is PushLike here
	})
	page.control(button("Print", margin, buttonY, "&Print")).on("MsiPrint", "1", condAlways)
	page.navigation()

	next := page.find(ctlNext)
	next.when("Disable", propLicense+` <> "1"`)
	next.when("Enable", propLicense+` = "1"`)

	w.checkBoxes[propLicense] = "1"
}

// license reads ui.license and returns it as RTF, or "" without a license.
func (b *build) license() string {
	src := b.cfg.UI.License
	if src == "" {
		return ""
	}

	data, err := b.readInput(src)
	if err != nil {
		b.errorf("ui.license", "%v", err)

		return ""
	}

	var text string

	if rtf.IsRTF(data) {
		text, err = rtf.Check(data)
	} else {
		text, err = rtf.FromText(data)
	}

	if err != nil {
		b.errorf("ui.license", "%s: %v", src, err)

		return ""
	}

	return text
}

// themePictures returns the banner and dialog background as bitmaps: the
// theme's own pictures, or the built-in ones in the theme's colours.
func (b *build) themePictures() ([]byte, []byte) {
	theme := &b.cfg.UI.Theme
	palette := themePalette(theme.Colors)

	banner := b.themePicture("ui.theme.banner", theme.Banner, palette, pictureSpec{
		units: [2]int{dlgWidth, bannerHeight}, pixels: [2]int{493, 58},
	})
	if banner == nil {
		banner = artwork.EncodeBMP(artwork.Banner(palette))
	}

	background := b.themePicture("ui.theme.dialog", theme.Dialog, palette, pictureSpec{
		units: [2]int{dlgWidth, stripY}, pixels: [2]int{493, 312},
	})
	if background == nil {
		background = artwork.EncodeBMP(artwork.Dialog(palette))
	}

	return banner, background
}

// pictureSpec is the control rectangle of a theme picture in dialog units
// and the pixel size that fills it at 96 dpi.
type pictureSpec struct {
	units  [2]int
	pixels [2]int
}

// themePicture reads and checks one theme picture and re-encodes it as a
// 24-bit bitmap, the form every Windows Installer version shows. It returns
// nil when the theme has no picture or the picture is unusable.
func (b *build) themePicture(path, src string, palette artwork.Palette, spec pictureSpec) []byte {
	if src == "" {
		return nil
	}

	data, err := b.readInput(src)
	if err != nil {
		b.errorf(path, "%v", err)

		return nil
	}

	img, err := artwork.Decode(data, palette.Paper)
	if err != nil {
		b.errorf(path, "%s: %v", src, err)

		return nil
	}

	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	want := float64(spec.units[0]) / float64(spec.units[1])
	got := float64(width) / float64(height)

	if math.Abs(got/want-1) > aspectTolerance {
		b.errorf(path, "%s is %d×%d pixels; the picture is stretched to %d×%d dialog units, so use %d×%d pixels or another size with that ratio",
			src, width, height, spec.units[0], spec.units[1], spec.pixels[0], spec.pixels[1])

		return nil
	}

	if width < spec.pixels[0] || height < spec.pixels[1] {
		b.warnf(path, "%s is %d×%d pixels and is enlarged on screen; use at least %d×%d pixels",
			src, width, height, spec.pixels[0], spec.pixels[1])
	}

	return artwork.EncodeBMP(img)
}

// themePalette is the default palette with the theme's colours applied.
// The colours were checked by config.Validate.
func themePalette(colors config.ThemeColors) artwork.Palette {
	palette := artwork.Default()

	for _, entry := range []struct {
		dst   *color.RGBA
		value string
	}{
		{&palette.Dark, colors.Dark}, {&palette.Light, colors.Light}, {&palette.Accent, colors.Accent}, {&palette.Paper, colors.Paper},
	} {
		if v, err := strconv.ParseUint(entry.value[min(1, len(entry.value)):], 16, 32); err == nil && len(entry.value) == 7 {
			*entry.dst = color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF} //nolint:gosec // 24-bit value
		}
	}

	return palette
}
