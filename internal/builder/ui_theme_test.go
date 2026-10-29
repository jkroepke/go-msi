package builder_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"maps"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/artwork"
	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

const licenseRTF = `{\rtf1\ansi\deff0{\fonttbl{\f0\fnil Courier New;}}\f0\fs16 Example License\par Version 1.0\par}`

// picturePNG is a width×height PNG in one colour.
func picturePNG(t *testing.T, width, height int, c color.NRGBA) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// brandedInputs are the input files of examples/branded, with the given
// overrides.
func brandedInputs(t *testing.T, overrides map[string][]byte) map[string][]byte {
	t.Helper()

	inputs := map[string][]byte{
		"agent.exe":         testpe.Build(testpe.Options{FileVersion: "1.5.0.0", Languages: []int{1033}}),
		"agent-service.exe": testpe.Build(testpe.Options{FileVersion: "1.5.0.0", Languages: []int{1033}}),
		"agent.conf":        []byte("logging {}\n"),
		"icon.ico":          append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...),
		"LICENSE.rtf":       []byte(licenseRTF),
		"banner.bmp":        picturePNG(t, 493, 58, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}),
		"dialog.bmp":        picturePNG(t, 493, 312, color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xFF}),
	}

	maps.Copy(inputs, overrides)

	return inputs
}

func controlRow(t *testing.T, res *builder.Result, dialog, control string) []any {
	t.Helper()

	for _, row := range rows(t, res.Database, "Control") {
		if row[0] == dialog && row[1] == control {
			return row
		}
	}

	t.Fatalf("%s has no control %s", dialog, control)

	return nil
}

func TestLicensePage(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, nil)))

	// Welcome → license → feature tree; an installed product's Change skips the license.
	for _, want := range []struct {
		dialog, control string
		next            map[string]string
	}{
		{"WelcomeDlg", "Next", map[string]string{"1": "LicenseAgreementDlg"}},
		{"LicenseAgreementDlg", "Back", map[string]string{"1": "WelcomeDlg"}},
		{"LicenseAgreementDlg", "Next", map[string]string{"1": "CustomizeDlg"}},
		{"CustomizeDlg", "Back", map[string]string{"NOT Installed": "LicenseAgreementDlg", "Installed": "MaintenanceTypeDlg"}},
		{"MaintenanceTypeDlg", "Change", map[string]string{"1": "CustomizeDlg"}},
	} {
		if got := nextOf(t, res, want.dialog, want.control); fmt.Sprint(got) != fmt.Sprint(want.next) {
			t.Errorf("%s %s goes to %v, want %v", want.dialog, want.control, got, want.next)
		}
	}

	if got := controlRow(t, res, "LicenseAgreementDlg", "LicenseText"); got[2] != "ScrollableText" || got[9] != licenseRTF {
		t.Errorf("license control %v", got)
	}

	if got := controlRow(t, res, "LicenseAgreementDlg", "LicenseAcceptedCheckBox"); got[2] != "CheckBox" || got[8] != "LicenseAccepted" {
		t.Errorf("check box %v", got)
	}

	if got := cell(t, res.Database, "CheckBox", "Value", func(r []any) bool { return r[0] == "LicenseAccepted" }); got != "1" {
		t.Errorf("CheckBox value %v", got)
	}

	conditions := map[string]string{}

	for _, row := range rows(t, res.Database, "ControlCondition") {
		if row[0] == "LicenseAgreementDlg" && row[1] == "Next" {
			conditions[fmt.Sprint(row[2])] = fmt.Sprint(row[3])
		}
	}

	if conditions["Disable"] != `LicenseAccepted <> "1"` || conditions["Enable"] != `LicenseAccepted = "1"` {
		t.Errorf("Next conditions %v", conditions)
	}

	found := false

	for _, row := range rows(t, res.Database, "ControlEvent") {
		if row[0] == "LicenseAgreementDlg" && row[1] == "Print" && row[2] == "MsiPrint" {
			found = true
		}
	}

	if !found {
		t.Error("Print publishes no MsiPrint")
	}

	checkRings(t, res)
}

func TestLicenseWithDialogs(t *testing.T) {
	t.Parallel()

	field := []config.DialogField{{Property: "PROP"}}

	for _, tc := range []struct {
		after string
		links map[[2]string]map[string]string
	}{
		{"WelcomeDlg", map[[2]string]map[string]string{
			{"WelcomeDlg", "Next"}:           {"1": "ExtraDlg"},
			{"ExtraDlg", "Back"}:             {"NOT Installed": "WelcomeDlg", "Installed": "MaintenanceTypeDlg"},
			{"ExtraDlg", "Next"}:             {"NOT Installed": "LicenseAgreementDlg", "Installed": "CustomizeDlg"},
			{"LicenseAgreementDlg", "Back"}:  {"1": "ExtraDlg"},
			{"CustomizeDlg", "Back"}:         {"NOT Installed": "LicenseAgreementDlg", "Installed": "ExtraDlg"},
			{"MaintenanceTypeDlg", "Change"}: {"1": "ExtraDlg"},
		}},
		{"LicenseAgreementDlg", map[[2]string]map[string]string{
			{"LicenseAgreementDlg", "Next"}:  {"1": "ExtraDlg"},
			{"ExtraDlg", "Back"}:             {"NOT Installed": "LicenseAgreementDlg", "Installed": "MaintenanceTypeDlg"},
			{"MaintenanceTypeDlg", "Change"}: {"1": "ExtraDlg"},
		}},
	} {
		cfg := example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, nil))
		cfg.UI.Dialogs = []*config.Dialog{{ID: "ExtraDlg", Title: "Extra", After: tc.after, Fields: field}}
		res := mustBuild(t, cfg)

		for at, want := range tc.links {
			if got := nextOf(t, res, at[0], at[1]); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("after %s: %s %s goes to %v, want %v", tc.after, at[0], at[1], got, want)
			}
		}

		checkRings(t, res)
	}
}

func TestLicenseFromText(t *testing.T) {
	t.Parallel()

	cfg := example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, map[string][]byte{"LICENSE": []byte("Copyright © Example\n{x}\n")}))
	cfg.UI.License = "LICENSE"
	res := mustBuild(t, cfg)

	got, ok := controlRow(t, res, "LicenseAgreementDlg", "LicenseText")[9].(string)
	if !ok || !strings.HasPrefix(got, `{\rtf1`) || !strings.Contains(got, `Copyright \u169? Example\par`) || !strings.Contains(got, `\{x\}`) {
		t.Errorf("converted license %q", got)
	}
}

func TestThemePictures(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, nil)))

	for name, want := range map[string]struct {
		width, height int
		red           uint8
	}{"BannerBmp": {493, 58, 0xFF}, "DialogBmp": {493, 312, 0x14}} {
		data, ok := cell(t, res.Database, "Binary", "Data", func(r []any) bool { return r[0] == name }).([]byte)
		if !ok {
			t.Fatalf("%s is not binary", name)
		}

		img, err := artwork.Decode(data, color.RGBA{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		// Always a 24-bit BITMAPINFOHEADER bitmap, whatever the input format.
		if data[28] != 24 || data[14] != 40 {
			t.Errorf("%s is %d bits with a %d-byte header", name, data[28], data[14])
		}

		if img.Bounds().Dx() != want.width || img.Bounds().Dy() != want.height || img.RGBAAt(5, 5).R != want.red {
			t.Errorf("%s is %v with %v", name, img.Bounds(), img.RGBAAt(5, 5))
		}
	}

	// The welcome page shows the whole background with the text over it.
	if got := controlRow(t, res, "WelcomeDlg", "Background"); got[2] != "Bitmap" || got[5] != 370 || got[6] != 234 || got[9] != "DialogBmp" {
		t.Errorf("background control %v", got)
	}

	if got := controlRow(t, res, "WelcomeDlg", "Title"); got[3] != 135 || got[7] != 0x10000|0x20000|3 {
		t.Errorf("title control %v", got)
	}
}

func TestThemeColors(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.UI.Wizard = "minimal"
	cfg.UI.Theme.Colors = config.ThemeColors{Dark: "#102030", Paper: "#fafafa"}
	res := mustBuild(t, cfg)

	data, ok := cell(t, res.Database, "Binary", "Data", func(r []any) bool { return r[0] == "DialogBmp" }).([]byte)
	if !ok {
		t.Fatal("DialogBmp is not binary")
	}

	img, err := artwork.Decode(data, color.RGBA{})
	if err != nil {
		t.Fatal(err)
	}

	if got := img.RGBAAt(0, 0); got != (color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}) {
		t.Errorf("panel corner %v", got)
	}

	if got := img.RGBAAt(artwork.DialogWidth-1, 0); got != (color.RGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF}) {
		t.Errorf("paper %v", got)
	}
}

func TestThemeAndLicenseErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		overrides map[string][]byte
		want      string
		warning   bool
	}{
		{"banner ratio", map[string][]byte{"banner.bmp": picturePNG(t, 493, 312, color.NRGBA{A: 0xFF})}, "stretched to 370×44 dialog units, so use 493×58", false},
		{"dialog not a picture", map[string][]byte{"dialog.bmp": []byte("GIF89a")}, "not a BMP, PNG or JPEG", false},
		{"small banner", map[string][]byte{"banner.bmp": picturePNG(t, 370, 44, color.NRGBA{A: 0xFF})}, "enlarged on screen", true},
		{"license not ASCII", map[string][]byte{"LICENSE.rtf": []byte("{\\rtf1 abc\xE9}")}, "byte 0xE9", false},
		{"license braces", map[string][]byte{"LICENSE.rtf": []byte(`{\rtf1 {x}`)}, "unclosed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, tc.overrides))
			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})

			if tc.warning != (err == nil) || !strings.Contains(fmt.Sprint(res.Diagnostics), tc.want) {
				t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
			}
		})
	}
}
