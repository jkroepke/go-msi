package builder

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
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
	// Bits above 0xFFFF mean different things per control type: on Text
	// 0x10000 is Transparent and 0x20000 NoPrefix, on ProgressBar 0x10000 is
	// Progress95, on CheckBox 0x20000 is PushLike, and 0x800000 is the
	// PushButton elevation shield.
	ctlTransp   = 0x10000
	ctlProg95   = 0x10000
	ctlFixedVol = 0x20000
	ctlRemoteV  = 0x40000
	ctlPassword = 0x200000
	// ctlErrorIcon is Visible | 0x80000 | FixedSize, the ErrorIcon value
	// tested on Windows. Microsoft documents 0x80000 as the Icon attribute
	// of buttons and IconSize32 as 0x400000; neither reading is verified.
	ctlErrorIcon = 0x180001

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
	sideTextX    = 135
	sideTextW    = 220
	stripY       = 234
	descriptionY = 90
	finishTextY  = 135
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
	radioStep    = 16
	// fieldsHeight is the room for fields between the banner and the
	// buttons: four label and edit pairs.
	fieldsHeight = 4 * (fieldLabelH + fieldEditH + fieldGap)

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
	binDialog = "DialogBmp"
	binError  = "ErrorIco"

	styleDefault = "DefaultFont"
	styleTitle   = "TitleFont"
	styleBig     = "BigFont"

	propBrowse   = "_BrowseProperty"
	propLicense  = "LicenseAccepted"
	propRMOption = "RMOPTION"
	propFilesUse = "FileInUseProcess"

	evNewDialog   = "NewDialog"
	evSpawnDialog = "SpawnDialog"
	evEndDialog   = "EndDialog"
	endReturn     = "Return"
	endExit       = "Exit"
	condAlways    = "1"

	typPushButton = "PushButton"
	typIcon       = "Icon"
	typListBox    = "ListBox"
	typDirCombo   = "DirectoryCombo"
	typDirList    = "DirectoryList"
	typPathEdit   = "PathEdit"
	typScrollText = "ScrollableText"
	typCheckBox   = "CheckBox"
	typText       = "Text"
	typRadioGroup = "RadioButtonGroup"
)

// uiDialog is one dialog with its controls in drawing order.
type uiDialog struct {
	wiz      *wizard
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

// uiRadio is a RadioButton row; a zero width or height is 330 by 16.
type uiRadio struct {
	property, value, text string
	y, w, h               int
}

// wizard collects the dialogs of one build.
type wizard struct {
	dialogs []*uiDialog
	byName  map[string]*uiDialog
	// ui holds ui.strings, which replace the built-in texts.
	ui     *config.UI
	radios []uiRadio
	// checkBoxes maps a CheckBox property to the value it sets.
	checkBoxes map[string]string
	// flow is the install path through the pages, from the welcome page
	// to the ready page; added dialogs are inserted into it.
	flow []string
}

// text returns the wizard text for a ui.strings key.
func (w *wizard) text(key string) string {
	return w.ui.UIString(key)
}

func (w *wizard) add(dialog *uiDialog) *uiDialog {
	if dialog.width == 0 {
		dialog.width, dialog.height = dlgWidth, dlgHeight
	}

	if dialog.title == "" {
		dialog.title = w.text("DialogTitle")
	}

	dialog.wiz = w
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
	return &uiControl{name: name, typ: typText, x: x, y: y, w: w, h: h, attrs: ctlText, text: s}
}

// label is a text with a mnemonic: without NoPrefix the engine underlines
// the letter after "&". Only for fixed texts, since an "&" from a property
// would be eaten.
func label(name string, x, y, w, h int, s string) *uiControl {
	return &uiControl{name: name, typ: typText, x: x, y: y, w: w, h: h, attrs: ctlActive, text: s}
}

func overlay(name string, x, y, w, h int, s string) *uiControl {
	return &uiControl{name: name, typ: typText, x: x, y: y, w: w, h: h, attrs: ctlOverlay, text: s}
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

	wiz := &wizard{byName: make(map[string]*uiDialog), checkBoxes: make(map[string]string), ui: b.languageUI()}
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

	if license := b.license(); license != "" {
		wiz.licenseDialog(license)
		wiz.flow = slices.Insert(wiz.flow, 1, dlgLicense)
	}

	if b.cfg.UI.InstallDirPage {
		wiz.installDirDialog(b.cfg.InstallDir.ID)
		wiz.flow = slices.Insert(wiz.flow, wiz.shown(1, 1), dlgInstallDir)
	}

	b.finishText(wiz)
	b.launch(wiz)

	b.addedDialogs(wiz)
	wiz.link()

	banner, background, errorIcon := b.themePictures()

	if err := b.diags.Err(); err != nil {
		return err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	b.table("Binary").Add(binBanner, banner)
	b.table("Binary").Add(binDialog, background)
	b.table("Binary").Add(binError, errorIcon)
	b.writeWizard(wiz)
	b.checkUI(wiz)

	return nil
}

// addedDialogs inserts the ui.dialogs pages into the flow after their anchor.
func (b *build) addedDialogs(wiz *wizard) {
	for i, cfg := range b.cfg.UI.Dialogs {
		path := fmt.Sprintf("ui.dialogs[%d]", i)

		// The names of the optional pages stay reserved without them,
		// since the flow links step over them by name.
		if _, taken := wiz.byName[cfg.ID]; taken || installOnly[cfg.ID] {
			b.errorf(path+".id", "%s is a wizard dialog", cfg.ID)

			continue
		}

		index := slices.Index(wiz.flow, cfg.After)
		if index < 0 || cfg.After == dlgVerifyReady {
			b.errorf(path+".after", "%s is not a wizard page that can be followed; use %s", cfg.After, strings.Join(wiz.flow[:len(wiz.flow)-1], ", "))

			continue
		}

		if height := formHeight(cfg); height > fieldsHeight {
			b.errorf(path+".fields", "the fields need %d dialog units of height and a dialog has %d (four text fields, "+
				"a choice takes 20 plus 16 per option); split them over two dialogs", height, fieldsHeight)

			continue
		}

		if !b.fieldCheckBoxes(wiz, path, cfg) {
			continue
		}

		wiz.add(b.formDialog(wiz, cfg))
		wiz.flow = slices.Insert(wiz.flow, index+1, cfg.ID)
	}
}

// fieldHeight is the height a field takes on a form dialog, without the gap
// below it.
func fieldHeight(field *config.DialogField) int {
	if field.Type == config.FieldChoice {
		return fieldLabelH + radioStep*len(field.Options)
	}

	return fieldLabelH + fieldEditH
}

// formHeight is the height the fields of a form dialog take.
func formHeight(cfg *config.Dialog) int {
	height := 0
	for i := range cfg.Fields {
		height += fieldHeight(&cfg.Fields[i]) + fieldGap
	}

	return height
}

// formDialog lays out an added dialog: per field a label and an edit, a
// check box in the same space, or a label and a group of radio buttons.
func (b *build) formDialog(wiz *wizard, cfg *config.Dialog) *uiDialog {
	dialog := &uiDialog{wiz: wiz, name: cfg.ID, attrs: dlgPage}
	dialog.innerHeader(cfg.Title, cfg.Description)

	top := fieldTop
	width := dlgWidth - 2*margin - 10

	for i := range cfg.Fields {
		field := &cfg.Fields[i]

		label := field.Label
		if label == "" {
			label = field.Property
		}

		var name string

		switch field.Type {
		case config.FieldCheckBox:
			name = fmt.Sprintf("CheckBox%d", i+1)
			dialog.control(&uiControl{
				name: name, typ: typCheckBox, property: field.Property, text: label, ring: true,
				x: margin + 5, y: top, w: width, h: fieldLabelH + fieldEditH, attrs: ctlActive,
			})
		case config.FieldChoice:
			name = fmt.Sprintf("Choice%d", i+1)
			dialog.control(text(fmt.Sprintf("Label%d", i+1), margin+5, top, width, fieldLabelH, label))
			dialog.control(&uiControl{
				name: name, typ: typRadioGroup, property: field.Property, ring: true,
				x: margin + 5, y: top + fieldLabelH, w: width, h: radioStep * len(field.Options), attrs: ctlActive,
			})
			b.choiceButtons(wiz, field, width)
		default:
			attrs := ctlActive
			if field.Type == config.FieldPassword {
				attrs |= ctlPassword
				b.hidden[field.Property] = true
			}

			name = fmt.Sprintf("Edit%d", i+1)
			dialog.control(text(fmt.Sprintf("Label%d", i+1), margin+5, top, width, fieldLabelH, label))
			dialog.control(&uiControl{
				name: name, typ: "Edit", property: field.Property, ring: true,
				x: margin + 5, y: top + fieldLabelH, w: width, h: fieldEditH, attrs: attrs,
			})
		}

		if dialog.first == "" {
			dialog.first = name
		}

		top += fieldHeight(field) + fieldGap
	}

	dialog.navigation()

	return dialog
}

// choiceButtons adds the radio buttons of a choice field; an option without
// a label shows its value, with & doubled so it stays literal. A property without
// a default selects the first option, because a group whose property matches
// no button fails ICE34 and starts with nothing selected.
func (b *build) choiceButtons(wiz *wizard, field *config.DialogField, width int) {
	for i, option := range field.Options {
		label := option.Label
		if label == "" {
			label = strings.ReplaceAll(option.Value, "&", "&&")
		}

		wiz.radios = append(wiz.radios, uiRadio{
			property: field.Property, value: option.Value, text: label,
			y: i * radioStep, w: width, h: radioStep,
		})
	}

	if _, set := b.properties[field.Property]; !set {
		b.properties[field.Property] = field.Options[0].Value
	}
}

// fieldCheckBoxes records the value each checkbox field of an added dialog
// sets; the CheckBox table holds one value per property.
func (b *build) fieldCheckBoxes(wiz *wizard, path string, cfg *config.Dialog) bool {
	for index := range cfg.Fields {
		field := &cfg.Fields[index]
		if field.Type != config.FieldCheckBox {
			continue
		}

		value := field.CheckBoxValue()
		if have, taken := wiz.checkBoxes[field.Property]; taken && have != value {
			b.errorf(fmt.Sprintf("%s.fields[%d].value", path, index), "another check box sets %s to %q", field.Property, have)

			return false
		}

		wiz.checkBoxes[field.Property] = value
	}

	return true
}

// innerHeader adds the banner with the page title and description.
func (d *uiDialog) innerHeader(title, description string) {
	d.control(bitmap("Banner", 0, 0, dlgWidth, bannerHeight, binBanner))
	d.control(overlay("Title", 15, 6, 300, 15, styled(styleTitle, title)))

	if description != "" {
		d.control(overlay("Description", 25, 21, 300, 22, description))
	}
}

// sideHeader adds the dialog background, whose left part is the side
// panel, with a large title and a description drawn over its right part.
func (d *uiDialog) sideHeader(title, description string) {
	d.control(bitmap("Background", 0, 0, dlgWidth, stripY, binDialog))
	d.control(overlay("Title", sideTextX, 20, sideTextW, 60, styled(styleBig, title)))
	d.control(overlay("Description", sideTextX, descriptionY, sideTextW, 80, description))
}

// navigation adds the bottom line with Back, Next and Cancel; Next and
// Back are linked by wizard.link.
func (d *uiDialog) navigation() {
	d.control(strip())
	d.control(button(ctlBack, backX, buttonY, d.wiz.text("Back")))
	d.control(button(ctlNext, nextX, buttonY, d.wiz.text("Next")))
	d.control(button(ctlCancel, cancelX, buttonY, d.wiz.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
	d.def = ctlNext
	d.cancel = ctlCancel
}

// link wires Next and Back along the flow. Change in the maintenance menu
// enters the flow at the first page after the welcome page that an
// installed product shows; that page goes back to the menu when the
// product is installed. An installed product steps over the license and
// install folder pages: the pages around them link to each other under
// Installed.
func (w *wizard) link() {
	entry := w.shown(1, 1)

	for i, name := range w.flow {
		page := w.byName[name]

		if i > 0 {
			back := page.find(ctlBack)

			switch {
			case i < entry:
				back.on(evNewDialog, w.flow[i-1], condAlways)
			case i == entry:
				back.on(evNewDialog, w.flow[i-1], condNotInstalled)
				back.on(evNewDialog, dlgMaintType, "Installed")
			default:
				step(back, w.flow[i-1], w.flow[w.shown(i-1, -1)])
			}
		}

		if i+1 < len(w.flow) {
			next := page.find(ctlNext)

			if i < entry {
				next.on(evNewDialog, w.flow[i+1], condAlways)
			} else {
				step(next, w.flow[i+1], w.flow[w.shown(i+1, 1)])
			}
		}
	}

	if change := w.byName[dlgMaintType].find("Change"); change != nil {
		change.on(evNewDialog, w.flow[entry], condAlways)
	}
}

// shown returns the index of the first page from flow[i] in direction dir
// that an installed product shows.
func (w *wizard) shown(i, dir int) int {
	for installOnly[w.flow[i]] {
		i += dir
	}

	return i
}

// step opens target, or installed for an installed product when the two
// differ.
func step(button *uiControl, target, installed string) {
	if target == installed {
		button.on(evNewDialog, target, condAlways)

		return
	}

	button.on(evNewDialog, target, condNotInstalled)
	button.on(evNewDialog, installed, "Installed")
}

// writeWizard emits the tables.
func (b *build) writeWizard(wiz *wizard) {
	styles := b.table("TextStyle")
	styles.Add(styleDefault, "Tahoma", 8, nil, nil)
	styles.Add(styleTitle, "Tahoma", 9, nil, 1)
	styles.Add(styleBig, "Tahoma", 13, nil, 1)

	b.properties["DefaultUIFont"] = styleDefault
	b.properties["ErrorDialog"] = dlgErrorName
	b.properties[propRMOption] = "UseRM"

	uiText := b.table("UIText")
	for _, key := range config.UITextKeys() {
		uiText.Add(key, wiz.text(key))
	}

	b.table("ListBox") // required with a Dialog table (ICE20), stays empty

	if len(wiz.checkBoxes) > 0 {
		boxes := b.table("CheckBox")
		for _, property := range sortedStringKeys(wiz.checkBoxes) {
			boxes.Add(property, wiz.checkBoxes[property])
		}
	}

	radios := b.table("RadioButton")
	for i, radio := range wiz.radios {
		width, height := radio.w, radio.h
		if width == 0 {
			width, height = 330, 16
		}

		radios.Add(radio.property, i+1, radio.value, 0, radio.y, width, height, nullable(radio.text), nil)
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
