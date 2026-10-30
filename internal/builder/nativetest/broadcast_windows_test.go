//go:build windows

package nativetest_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

//nolint:gochecknoglobals // lazy procedures
var (
	pRegisterClassExW          = user32.NewProc("RegisterClassExW")
	pCreateWindowExW           = user32.NewProc("CreateWindowExW")
	pDefWindowProcW            = user32.NewProc("DefWindowProcW")
	pGetMessageW               = user32.NewProc("GetMessageW")
	pDispatchMessageW          = user32.NewProc("DispatchMessageW")
	pPostMessageW              = user32.NewProc("PostMessageW")
	pPostQuitMessage           = user32.NewProc("PostQuitMessage")
	pSendMessageTimeoutW       = user32.NewProc("SendMessageTimeoutW")
	pOpenProcess               = kernel32.NewProc("OpenProcess")
	pReadProcessMemory         = kernel32.NewProc("ReadProcessMemory")
	pNtQueryInformationProcess = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQueryInformationProcess")
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmSettingChange = 0x001A
	// broadcastSettle is how long a broadcast may trail msiexec's exit.
	broadcastSettle = 5 * time.Second
)

// settingChanges records the WM_SETTINGCHANGE messages that a hidden
// top-level window receives; HWND_BROADCAST reaches every top-level
// window, visible or not.
type settingChanges struct {
	mu   sync.Mutex
	seen []string
	hwnd uintptr
	done chan struct{}
}

//nolint:gochecknoglobals // one window procedure for the process: callbacks are never freed
var (
	listener     *settingChanges
	listenerProc = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
			switch msg {
			case wmSettingChange:
				listener.record(wParam, lParam)

				return 0
			case wmDestroy:
				_, _, _ = pPostQuitMessage.Call(0)

				return 0
			}

			r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)

			return r
		})
	})
)

// listenSettingChanges starts the window on its own locked thread.
func listenSettingChanges(t *testing.T) *settingChanges {
	t.Helper()

	probe := &settingChanges{done: make(chan struct{})}
	listener = probe
	ready := make(chan error, 1)

	go func() {
		runtime.LockOSThread()

		defer close(probe.done)

		name, _ := syscall.UTF16PtrFromString("MsiBuilderSettingChangeProbe")

		class := struct {
			size, style                    uint32
			proc                           uintptr
			clsExtra, wndExtra             int32
			instance, icon, cursor, brush  uintptr
			menuName, className, smallIcon uintptr
		}{proc: listenerProc(), className: uintptr(unsafe.Pointer(name))}
		class.size = uint32(unsafe.Sizeof(class))

		if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&class))); r == 0 && err != syscall.Errno(1410) { //nolint:errorlint // class already registered
			ready <- fmt.Errorf("register window class: %w", err)

			return
		}

		hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, 0, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("create window: %w", err)

			return
		}

		probe.hwnd = hwnd

		ready <- nil

		var msg [48]byte
		for {
			r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || r == ^uintptr(0) {
				return
			}

			_, _, _ = pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()

	if err := <-ready; err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _, _ = pPostMessageW.Call(probe.hwnd, wmClose, 0, 0)
		<-probe.done
	})

	return probe
}

func (l *settingChanges) record(wParam, lParam uintptr) {
	area := "<null>"
	if lParam != 0 {
		area = utf16At(*(**uint16)(unsafe.Pointer(&lParam)), 512)
	}

	l.mu.Lock()
	l.seen = append(l.seen, fmt.Sprintf("wParam=%d lParam=%q", wParam, area))
	l.mu.Unlock()
}

// take returns and clears what the window received so far.
func (l *settingChanges) take() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	seen := l.seen
	l.seen = nil

	return seen
}

// utf16At reads a NUL-terminated string of at most limit units.
func utf16At(text *uint16, limit int) string {
	n := 0
	for n < limit && *(*uint16)(unsafe.Add(unsafe.Pointer(text), 2*n)) != 0 {
		n++
	}

	return syscall.UTF16ToString(unsafe.Slice(text, n))
}

// explorerPID returns the PID of the shell in this session, or 0.
func explorerPID(t *testing.T) int {
	t.Helper()

	const ps = `$s = (Get-Process -Id $PID).SessionId; Get-Process explorer -ErrorAction SilentlyContinue | Where-Object SessionId -eq $s | Select-Object -First 1 -ExpandProperty Id`

	out, err := exec.CommandContext(t.Context(), "powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return 0
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))

	return pid
}

// processEnvironment reads the current environment block of a 64-bit
// process through its PEB: ProcessParameters at PEB+0x20, Environment at
// ProcessParameters+0x80. SetEnvironmentStrings swaps that pointer, so it
// shows what a program the process starts next would inherit.
func processEnvironment(pid int) (map[string]string, error) {
	const queryInformation, vmRead = 0x0400, 0x0010

	process, _, err := pOpenProcess.Call(queryInformation|vmRead, 0, uintptr(pid))
	if process == 0 {
		return nil, fmt.Errorf("open process: %w", err)
	}

	defer syscall.CloseHandle(syscall.Handle(process)) //nolint:errcheck // read-only handle

	var info [6]uintptr // PROCESS_BASIC_INFORMATION

	if r, _, _ := pNtQueryInformationProcess.Call(process, 0, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), 0); r != 0 {
		return nil, fmt.Errorf("query process information: 0x%x", r)
	}

	read := func(addr uintptr, buf []byte) error {
		var n uintptr

		r, _, err := pReadProcessMemory.Call(process, addr, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)))
		if r == 0 {
			return fmt.Errorf("read process memory: %w", err)
		}

		return nil
	}

	pointer := func(addr uintptr) (uintptr, error) {
		var buf [8]byte
		if err := read(addr, buf[:]); err != nil {
			return 0, err
		}

		return *(*uintptr)(unsafe.Pointer(&buf[0])), nil
	}

	params, err := pointer(info[1] + 0x20)
	if err != nil {
		return nil, err
	}

	block, err := pointer(params + 0x80)
	if err != nil {
		return nil, err
	}

	// The block size field may lag a swapped block, so read pages until
	// the double NUL that ends it.
	var data []byte

	page := make([]byte, 4096-block%4096)
	for len(data) < 4<<20 {
		if err := read(block+uintptr(len(data)), page); err != nil {
			break
		}

		data = append(data, page...)
		if end := endOfBlock(data); end >= 0 {
			data = data[:end]

			break
		}

		page = make([]byte, 4096)
	}

	units := unsafe.Slice((*uint16)(unsafe.Pointer(unsafe.SliceData(data))), len(data)/2)
	env := map[string]string{}

	for entry := range strings.SplitSeq(syscall.UTF16ToString(slices.Clone(units)), "\x00") {
		if i := strings.Index(entry[1:], "="); i >= 0 {
			env[strings.ToUpper(entry[:i+1])] = entry[i+2:]
		}
	}

	return env, nil
}

// endOfBlock returns the byte offset of an environment block's closing
// double NUL, or -1.
func endOfBlock(data []byte) int {
	for i := 0; i+3 < len(data); i += 2 {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
			return i + 2
		}
	}

	return -1
}

const broadcastConfig = `schema: msi-builder/v1
product: { name: MsiBuilderBroadcast%[1]s, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: %[4]s }
package: { platform: x64, scope: %[2]s }
installDir: { name: MsiBuilderBroadcast%[1]s }
components:
  - files: [{ source: app.txt }]
%[3]s`

// TestMsiexecEnvironmentBroadcast probes whether msiexec announces
// Environment changes with WM_SETTINGCHANGE (W16, ENVVAR-02): a hidden
// top-level window logs every WM_SETTINGCHANGE while packages that set a
// machine variable, a user variable from a per-machine package, a user
// variable from a per-user package, and no variable at all are installed
// and removed. Where the session has a shell, the test also reads
// Explorer's environment block, which is what a program started from the
// Start menu or Run inherits.
func TestMsiexecEnvironmentBroadcast(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	if !elevated() {
		t.Skip("per-machine installs need an elevated process")
	}

	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("the PEB offsets are those of 64-bit processes")
	}

	waitForMsiexec(t)

	changes := listenSettingChanges(t)
	shell := explorerPID(t)
	t.Logf("explorer PID in this session: %d", shell)

	if shell != 0 {
		env, err := processEnvironment(shell)
		t.Logf("shell environment: %d variables, Path set %t, SystemRoot=%q, error %v", len(env), env["PATH"] != "", env["SYSTEMROOT"], err)
	}

	shellSees := func(name string) string {
		if shell == 0 {
			return "no shell"
		}

		env, err := processEnvironment(shell)
		if err != nil {
			return err.Error()
		}

		value, ok := env[strings.ToUpper(name)]
		if !ok {
			return "<absent>"
		}

		return strconv.Quote(value)
	}

	// A user variable written to the registry without any broadcast: the
	// shell must not see it until something broadcasts.
	const raw = "MSIBUILDER_UNANNOUNCED"
	if out, err := exec.CommandContext(t.Context(), "reg.exe", "add", `HKCU\Environment`, "/v", raw, "/d", "1", "/f").CombinedOutput(); err != nil {
		t.Fatalf("reg add: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		_ = exec.Command("reg.exe", "delete", `HKCU\Environment`, "/v", raw, "/f").Run() //nolint:noctx // cleanup after the test context ends
	})

	time.Sleep(broadcastSettle)
	t.Logf("after a plain reg add: messages %q, shell sees %s=%s", changes.take(), raw, shellSees(raw))

	for _, tc := range []struct { //nolint:paralleltest // msiexec runs one installation at a time
		name, scope, env, variable, key, upgradeCode string
	}{
		{"Machine", "perMachine", "machine", "MSIBUILDER_MACHINE", `HKLM\System\CurrentControlSet\Control\Session Manager\Environment`, "5b0e7c3a-2f41-4d8e-9a16-c7e2b4f80d51"},
		{"UserFromMachine", "perMachine", "user", "MSIBUILDER_USER_PM", `HKCU\Environment`, "a3d94f12-6c8b-4e70-b5a2-1f9e0c7d3b64"},
		{"UserFromUser", "perUser", "user", "MSIBUILDER_USER_PU", `HKCU\Environment`, "e81c2b57-04d9-4a3f-8e6b-9d2a5c1f7e08"},
		{"NoVariable", "perMachine", "", "", "", "2c7f5e90-b38a-4d16-a0e4-6b1d9f3c8a27"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment := ""
			if tc.env != "" {
				environment = fmt.Sprintf("    environment: [{ name: %s, value: '[INSTALLDIR]', scope: %s }]\n", tc.variable, tc.env)
			}

			dir := t.TempDir()
			path := buildConfig(t, dir, "broadcast", fmt.Sprintf(broadcastConfig, tc.name, tc.scope, environment, tc.upgradeCode), map[string][]byte{"app.txt": []byte("app\n")})

			t.Cleanup(func() {
				_ = exec.Command("msiexec", "/x", path, "/qn").Run() //nolint:noctx // cleanup after the test context ends
			})

			step := func(verb string, args ...string) {
				log := filepath.Join(dir, verb+".log")

				changes.take()

				start := time.Now()

				if code := msiexec(t, append(args, "/qn", "/l*v", log)...); code != 0 {
					t.Fatalf("msiexec %s: exit %d", verb, code)
				}

				took := time.Since(start)

				time.Sleep(broadcastSettle)

				registry := "-"
				if tc.key != "" {
					registry = strconv.FormatBool(regHas(t, "64", tc.key, tc.variable))
				}

				t.Logf("%s (%s, %s): msiexec %s; messages %q; registry has it: %s; shell sees %s=%s, %s=%s",
					verb, tc.scope, tc.env, took.Round(time.Millisecond), changes.take(), registry,
					tc.variable, shellSees(tc.variable), raw, shellSees(raw))

				if data, err := os.ReadFile(log); err == nil {
					for line := range strings.SplitSeq(string(data), "\n") {
						if strings.Contains(line, "Environment") && !strings.Contains(line, "Property(") {
							t.Logf("%s log: %s", verb, strings.TrimSpace(strings.ReplaceAll(line, "\x00", "")))
						}
					}
				}
			}

			step("install", "/i", path)
			step("repair", "/fvamus", path)
			step("uninstall", "/x", path)
		})
	}

	// The broadcast setx and WiX's custom action send, from this process.
	area, _ := syscall.UTF16PtrFromString("Environment")

	var result uintptr

	const hwndBroadcast, smtoAbortIfHung = 0xffff, 0x0002

	r, _, err := pSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(area)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))

	time.Sleep(broadcastSettle)
	t.Logf("own broadcast: SendMessageTimeoutW %d (%v); messages %q; shell sees %s=%s", r, err, changes.take(), raw, shellSees(raw))
}
