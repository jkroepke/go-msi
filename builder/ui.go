package builder

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/go-msi/artwork"
	"github.com/jkroepke/go-msi/config"
)

// The wizard is pure table data (spec section 7): dialogs hold controls,
// controls publish events, and the UI sequences open the first page. Every
// page is 370×270 installer units with the buttons on a bottom strip; inner
// pages carry a banner, the welcome and end pages a side panel.
const (
	dlgWidth  = 370
	dlgHeight = 270

	// Dialog attribute bits.
	dlgVisible        = 1
	dlgModal          = 2
	dlgMinimize       = 4
	dlgKeepModeless   = 16
	dlgTrackDiskSpace = 32
	dlgError          = 65536

	dlgPage     = dlgVisible | dlgModal | dlgMinimize
	dlgProgress = dlgVisible | dlgMinimize
	dlgFilesUse = dlgPage | dlgKeepModeless
	dlgCosting  = dlgPage | dlgTrackDiskSpace
	dlgErrorBox = dlgPage | dlgError

	// Control attribute bits.
	ctlVisible  = 1
	ctlEnabled  = 2
	ctlSunken   = 4
	ctlIndirect = 8
	ctlNoPrefix = 0x20000
	ctlTransp   = 0x800000
	ctlProg95   = 0x10000
	ctlFixedVol = 0x20000
	ctlRemoteV  = 0x40000

	ctlActive   = ctlVisible | ctlEnabled
	ctlText     = ctlActive | ctlNoPrefix
	ctlOverlay  = ctlText | ctlTransp
	ctlDisabled = ctlVisible
	ctlList     = ctlActive | ctlSunken
	ctlVolumes  = ctlList | ctlFixedVol | ctlRemoteV
	ctlDirCombo = ctlActive | ctlIndirect | ctlFixedVol | ctlRemoteV
	ctlDirList  = ctlList | ctlIndirect
	ctlPathEdit = ctlActive | ctlIndirect
	ctlProgress = ctlVisible | ctlProg95

	// Layout.
	bannerHeight = 44
	sideWidth    = 164
	stripY       = 234
	buttonY      = 243
	buttonW      = 56
	buttonH      = 17
	backX        = 180
	nextX        = 236
	cancelX      = 304
	margin       = 20
	fieldLabelH  = 20
	fieldEditH   = 15
	fieldGap     = 5
	fieldTop     = 58
	maxFields    = 4

	// Keys shared by several pages.
	dlgWelcome      = "WelcomeDlg"
	dlgCustomize    = "CustomizeDlg"
	dlgVerifyReady  = "VerifyReadyDlg"
	dlgProgressName = "ProgressDlg"
	dlgCancel       = "CancelDlg"
	dlgErrorName    = "ErrorDlg"
	dlgMaintType    = "MaintenanceTypeDlg"
	dlgMaintWelcome = "MaintenanceWelcomeDlg"
	dlgWaitCosting  = "WaitForCostingDlg"
	dlgBrowse       = "BrowseDlg"
	dlgDiskCost     = "DiskCostDlg"
	dlgExit         = "ExitDlg"
	dlgUserExit     = "UserExitDlg"
	dlgFatal        = "FatalErrorDlg"
	dlgResume       = "ResumeDlg"
	dlgFilesInUse   = "FilesInUse"
	dlgRMFilesInUse = "MsiRMFilesInUse"
	dlgVerifyRepair = "VerifyRepairDlg"
	dlgVerifyRemove = "VerifyRemoveDlg"

	ctlBack   = "Back"
	ctlNext   = "Next"
	ctlCancel = "Cancel"

	binBanner = "BannerBmp"
	binSide   = "SideBmp"

	styleDefault = "DefaultFont"
	styleTitle   = "TitleFont"
	styleBig     = "BigFont"

	propBrowse   = "_BrowseProperty"
	propRMOption = "RMOPTION"
	propFilesUse = "FileInUseProcess"

	evNewDialog   = "NewDialog"
	evSpawnDialog = "SpawnDialog"
	evEndDialog   = "EndDialog"
	endReturn     = "Return"
	endExit       = "Exit"
	condAlways    = "1"

	typPushButton = "PushButton"
	typListBox    = "ListBox"
	typDirCombo   = "DirectoryCombo"
	typDirList    = "DirectoryList"
	typPathEdit   = "PathEdit"
)

// uiDialog is one dialog with its controls in drawing order.
type uiDialog struct {
	name     string
	title    string
	first    string
	def      string
	cancel   string
	controls []*uiControl
	attrs    int
	width    int
	height   int
}

// uiControl is one Control row with its events and subscriptions.
type uiControl struct {
	name     string
	typ      string
	property string
	text     string
	events   []uiEvent
	subs     []uiSub
	conds    []uiCond
	x, y     int
	w, h     int
	attrs    int
	ring     bool
}

type uiEvent struct{ event, arg, cond string }

type uiSub struct{ event, attribute string }

type uiCond struct{ action, cond string }

// uiRadio is a RadioButton row.
type uiRadio struct {
	property, value, text string
	y                     int
}

// wizard collects the dialogs of one build.
type wizard struct {
	dialogs []*uiDialog
	byName  map[string]*uiDialog
	radios  []uiRadio
	// flow is the install path through the pages, from the welcome page
	// to the ready page; added dialogs are inserted into it.
	flow []string
}

func (w *wizard) add(dialog *uiDialog) *uiDialog {
	if dialog.width == 0 {
		dialog.width, dialog.height = dlgWidth, dlgHeight
	}

	if dialog.title == "" {
		dialog.title = "[ProductName] Setup"
	}

	w.dialogs = append(w.dialogs, dialog)
	w.byName[dialog.name] = dialog

	return dialog
}

func (d *uiDialog) control(c *uiControl) *uiControl {
	d.controls = append(d.controls, c)

	return c
}

func (d *uiDialog) find(name string) *uiControl {
	for _, c := range d.controls {
		if c.name == name {
			return c
		}
	}

	return nil
}

func (c *uiControl) on(event, arg, cond string) *uiControl {
	c.events = append(c.events, uiEvent{event: event, arg: arg, cond: cond})

	return c
}

func (c *uiControl) subscribe(event, attribute string) *uiControl {
	c.subs = append(c.subs, uiSub{event: event, attribute: attribute})

	return c
}

func (c *uiControl) when(action, cond string) {
	c.conds = append(c.conds, uiCond{action: action, cond: cond})
}

// Control constructors.

func text(name string, x, y, w, h int, s string) *uiControl {
	return &uiControl{name: name, typ: "Text", x: x, y: y, w: w, h: h, attrs: ctlText, text: s}
}

func overlay(name string, x, y, w, h int, s string) *uiControl {
	return &uiControl{name: name, typ: "Text", x: x, y: y, w: w, h: h, attrs: ctlOverlay, text: s}
}

func button(name string, x, y int, s string) *uiControl {
	return &uiControl{name: name, typ: typPushButton, x: x, y: y, w: buttonW, h: buttonH, attrs: ctlActive, text: s, ring: true}
}

// disabledButton is a greyed button on the bottom strip.
func disabledButton(name string, x int, s string) *uiControl {
	return &uiControl{name: name, typ: typPushButton, x: x, y: buttonY, w: buttonW, h: buttonH, attrs: ctlDisabled, text: s}
}

func bitmap(name string, x, y, w, h int, binaryKey string) *uiControl {
	return &uiControl{name: name, typ: "Bitmap", x: x, y: y, w: w, h: h, attrs: ctlVisible, text: binaryKey}
}

// strip is the line above the buttons.
func strip() *uiControl {
	return &uiControl{name: "BottomLine", typ: "Line", x: 0, y: stripY, w: dlgWidth, h: 0, attrs: ctlVisible}
}

func styled(style, s string) string {
	return "{\\" + style + "}" + s
}

// ui writes the wizard tables when a wizard is selected (spec section 7).
func (b *build) ui() error {
	if b.cfg.UI.Wizard == "none" {
		if _, set := b.properties["ARPNOMODIFY"]; !set {
			b.properties["ARPNOMODIFY"] = "1"
		}

		return nil
	}

	wiz := &wizard{byName: make(map[string]*uiDialog)}
	tree := b.cfg.UI.Wizard == "featureTree"

	wiz.commonDialogs(tree, b.hasRows("Component"))

	if tree {
		wiz.featureDialogs()
		wiz.flow = []string{dlgWelcome, dlgCustomize, dlgVerifyReady}
	} else {
		wiz.flow = []string{dlgWelcome, dlgVerifyReady}

		if _, set := b.properties["ARPNOMODIFY"]; !set {
			b.properties["ARPNOMODIFY"] = "1"
		}
	}

	b.addedDialogs(wiz)
	wiz.link()

	if err := b.diags.Err(); err != nil {
		return err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	b.writeWizard(wiz)
	b.checkUI(wiz)

	return nil
}

// addedDialogs inserts the ui.dialogs pages into the flow after their anchor.
func (b *build) addedDialogs(wiz *wizard) {
	for i, cfg := range b.cfg.UI.Dialogs {
		path := fmt.Sprintf("ui.dialogs[%d]", i)

		if _, taken := wiz.byName[cfg.ID]; taken {
			b.errorf(path+".id", "%s is a wizard dialog", cfg.ID)

			continue
		}

		index := slices.Index(wiz.flow, cfg.After)
		if index < 0 || cfg.After == dlgVerifyReady {
			b.errorf(path+".after", "%s is not a wizard page that can be followed; use %s", cfg.After, strings.Join(wiz.flow[:len(wiz.flow)-1], ", "))

			continue
		}

		if len(cfg.Fields) > maxFields {
			b.errorf(path+".fields", "a dialog holds at most %d fields; split them over two dialogs", maxFields)

			continue
		}

		wiz.add(formDialog(cfg))
		wiz.flow = slices.Insert(wiz.flow, index+1, cfg.ID)
	}
}

// formDialog lays out an added dialog: one label and one edit per field.
func formDialog(cfg *config.Dialog) *uiDialog {
	dialog := &uiDialog{name: cfg.ID, attrs: dlgPage}
	dialog.innerHeader(cfg.Title, cfg.Description)

	top := fieldTop

	for i, field := range cfg.Fields {
		label := field.Label
		if label == "" {
			label = field.Property
		}

		dialog.control(text(fmt.Sprintf("Label%d", i+1), margin+5, top, dlgWidth-2*margin-10, fieldLabelH, label))
		dialog.control(&uiControl{
			name: fmt.Sprintf("Edit%d", i+1), typ: "Edit", property: field.Property, ring: true,
			x: margin + 5, y: top + fieldLabelH, w: dlgWidth - 2*margin - 10, h: fieldEditH, attrs: ctlActive,
		})

		top += fieldLabelH + fieldEditH + fieldGap
	}

	dialog.first = "Edit1"
	dialog.navigation()

	return dialog
}

// innerHeader adds the banner with the page title and description.
func (d *uiDialog) innerHeader(title, description string) {
	d.control(bitmap("Banner", 0, 0, dlgWidth, bannerHeight, binBanner))
	d.control(overlay("Title", 15, 6, 300, 15, styled(styleTitle, title)))

	if description != "" {
		d.control(overlay("Description", 25, 23, 320, 18, description))
	}
}

// sideHeader adds the side panel with a large title and a description.
func (d *uiDialog) sideHeader(title, description string) {
	d.control(bitmap("Side", 0, 0, sideWidth, stripY, binSide))
	d.control(text("Title", sideWidth+16, 20, dlgWidth-sideWidth-30, 60, styled(styleBig, title)))
	d.control(text("Description", sideWidth+16, 90, dlgWidth-sideWidth-30, 80, description))
}

// navigation adds the bottom line with Back, Next and Cancel; Next and
// Back are linked by wizard.link.
func (d *uiDialog) navigation() {
	d.control(strip())
	d.control(button(ctlBack, backX, buttonY, "< &Back"))
	d.control(button(ctlNext, nextX, buttonY, "&Next >"))
	d.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
	d.def = ctlNext
	d.cancel = ctlCancel
}

// link wires Next and Back along the flow. The page after the welcome page
// goes back to the maintenance menu when the product is installed, because
// Change enters the flow there.
func (w *wizard) link() {
	for i, name := range w.flow {
		page := w.byName[name]

		if i > 0 {
			back := page.find(ctlBack)
			if i == 1 {
				back.on(evNewDialog, w.flow[0], condNotInstalled)
				back.on(evNewDialog, dlgMaintType, "Installed")
			} else {
				back.on(evNewDialog, w.flow[i-1], condAlways)
			}
		}

		if i+1 < len(w.flow) {
			page.find(ctlNext).on(evNewDialog, w.flow[i+1], condAlways)
		}
	}

	if change := w.byName[dlgMaintType].find("Change"); change != nil {
		change.on(evNewDialog, w.flow[1], condAlways)
	}
}

// writeWizard emits the tables.
func (b *build) writeWizard(wiz *wizard) {
	b.table("Binary").Add(binBanner, artwork.EncodeBMP(artwork.Banner(artwork.Default())))
	b.table("Binary").Add(binSide, artwork.EncodeBMP(artwork.Side(artwork.Default())))

	styles := b.table("TextStyle")
	styles.Add(styleDefault, "Tahoma", 8, nil, nil)
	styles.Add(styleTitle, "Tahoma", 9, nil, 1)
	styles.Add(styleBig, "Tahoma", 13, nil, 1)

	b.properties["DefaultUIFont"] = styleDefault
	b.properties["ErrorDialog"] = dlgErrorName
	b.properties[propRMOption] = "UseRM"

	uiText := b.table("UIText")
	for _, kv := range uiTexts {
		uiText.Add(kv[0], kv[1])
	}

	b.table("ListBox") // required with a Dialog table (ICE20), stays empty

	radios := b.table("RadioButton")
	for i, r := range wiz.radios {
		radios.Add(r.property, i+1, r.value, 0, r.y, 330, 16, r.text, nil)
	}

	slices.SortFunc(wiz.dialogs, func(a, b *uiDialog) int { return strings.Compare(a.name, b.name) })

	dialogs := b.table("Dialog")

	for _, dialog := range wiz.dialogs {
		dialogs.Add(dialog.name, 50, 50, dialog.width, dialog.height, dialog.attrs, dialog.title, dialog.first, nullable(dialog.def), nullable(dialog.cancel))
		b.writeControls(dialog)
	}

	b.scheduleDialogs()
}

// writeControls emits the Control, ControlEvent, EventMapping and
// ControlCondition rows of one dialog. Control_Next links the ring controls
// in drawing order and closes the loop (ICE23).
func (b *build) writeControls(dialog *uiDialog) {
	controls := b.table("Control")
	events := b.table("ControlEvent")
	mappings := b.table("EventMapping")
	conditions := b.table("ControlCondition")

	ring := make([]*uiControl, 0, len(dialog.controls))

	for _, ctl := range dialog.controls {
		if ctl.ring {
			ring = append(ring, ctl)
		}
	}

	for _, ctl := range dialog.controls {
		var next any

		if i := slices.Index(ring, ctl); i >= 0 {
			next = ring[(i+1)%len(ring)].name
		}

		controls.Add(dialog.name, ctl.name, ctl.typ, ctl.x, ctl.y, ctl.w, ctl.h, ctl.attrs, nullable(ctl.property), nullable(ctl.text), next, nil)

		for i, ev := range ctl.events {
			events.Add(dialog.name, ctl.name, ev.event, ev.arg, ev.cond, i+1)
		}

		for _, sub := range ctl.subs {
			mappings.Add(dialog.name, ctl.name, sub.event, sub.attribute)
		}

		for _, cond := range ctl.conds {
			conditions.Add(dialog.name, ctl.name, cond.action, cond.cond)
		}
	}
}

// scheduleDialogs puts the entry pages and the end dialogs into both UI
// sequences (spec sections 7.3 and 10.3).
func (b *build) scheduleDialogs() {
	b.schedule(seqInstallUI, dlgWelcome, condNotInstalled, "", "", 1230)
	b.schedule(seqInstallUI, dlgResume, "Installed AND (RESUME OR Preselected)", "", "", 1240)
	b.schedule(seqInstallUI, dlgMaintWelcome, "Installed AND NOT RESUME AND NOT Preselected AND NOT PATCH", "", "", 1250)

	for _, table := range []string{seqInstallUI, seqAdminUI} {
		b.schedule(table, dlgProgressName, "", "", "", 1280)
		b.schedule(table, dlgExit, "", "", "", -1)
		b.schedule(table, dlgUserExit, "", "", "", -2)
		b.schedule(table, dlgFatal, "", "", "", -3)
	}
}

// isDialog reports whether name is a dialog of this build.
func (b *build) isDialog(name string) bool {
	tbl, ok := b.tables["Dialog"]
	if !ok {
		return false
	}

	for _, row := range tbl.Rows {
		if row[0] == name {
			return true
		}
	}

	return false
}
