// gomsirun.c is the custom action DLL that go-msi embeds into packages for
// `run` actions. The entry point GoMsiRun starts a command line with a
// hidden window, copies everything the process writes to stdout or stderr
// into the Windows Installer log through MsiProcessMessage, and turns the
// exit code into a custom action return value: 0 for exit code 0,
// ERROR_INSTALL_FAILURE for anything else.
//
// The DLL is freestanding: it includes no SDK header and links no C runtime,
// only kernel32.dll, advapi32.dll and msi.dll, whose prototypes are written
// out below from Microsoft's documentation of each function. It is built by
// build.sh with clang and lld for x86, x64 and arm64.
//
// Input, in CustomActionData for deferred, rollback and commit actions and in
// the property GoMsiRun for immediate actions:
//
//     <n>:<working directory><command line>
//
// where <n> is the decimal length in UTF-16 units of the working directory,
// which may be empty. The command line follows directly after the directory.
// A log line reads `GoMsiRun: ...`.

typedef unsigned int UINT;
typedef unsigned long DWORD;
typedef int BOOL;
typedef void *HANDLE;
typedef unsigned short WCHAR;
typedef unsigned char BYTE;
typedef unsigned long MSIHANDLE;
typedef unsigned long long SIZE_T_;

#if defined(__i386__)
#define WINAPI __attribute__((stdcall))
typedef unsigned int SIZE_T;
#else
#define WINAPI
typedef SIZE_T_ SIZE_T;
#endif

typedef struct {
	DWORD nLength;
	void *lpSecurityDescriptor;
	BOOL bInheritHandle;
} SECURITY_ATTRIBUTES;

typedef struct {
	DWORD cb;
	WCHAR *lpReserved;
	WCHAR *lpDesktop;
	WCHAR *lpTitle;
	DWORD dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
	unsigned short wShowWindow, cbReserved2;
	BYTE *lpReserved2;
	HANDLE hStdInput, hStdOutput, hStdError;
} STARTUPINFOW;

typedef struct {
	HANDLE hProcess, hThread;
	DWORD dwProcessId, dwThreadId;
} PROCESS_INFORMATION;

// kernel32.dll
BOOL WINAPI CreatePipe(HANDLE *, HANDLE *, SECURITY_ATTRIBUTES *, DWORD);
BOOL WINAPI SetHandleInformation(HANDLE, DWORD, DWORD);
BOOL WINAPI CreateProcessW(const WCHAR *, WCHAR *, SECURITY_ATTRIBUTES *, SECURITY_ATTRIBUTES *, BOOL, DWORD, void *, const WCHAR *, STARTUPINFOW *, PROCESS_INFORMATION *);
BOOL WINAPI ReadFile(HANDLE, void *, DWORD, DWORD *, void *);
DWORD WINAPI WaitForSingleObject(HANDLE, DWORD);
BOOL WINAPI GetExitCodeProcess(HANDLE, DWORD *);
BOOL WINAPI CloseHandle(HANDLE);
HANDLE WINAPI GetCurrentThread(void);
HANDLE WINAPI GetProcessHeap(void);
void *WINAPI HeapAlloc(HANDLE, DWORD, SIZE_T);
void *WINAPI HeapReAlloc(HANDLE, DWORD, void *, SIZE_T);
BOOL WINAPI HeapFree(HANDLE, DWORD, void *);
DWORD WINAPI GetLastError(void);
int WINAPI MultiByteToWideChar(UINT, DWORD, const char *, int, WCHAR *, int);
BOOL WINAPI TerminateProcess(HANDLE, UINT);

// advapi32.dll
BOOL WINAPI OpenThreadToken(HANDLE, DWORD, BOOL, HANDLE *);
BOOL WINAPI DuplicateTokenEx(HANDLE, DWORD, SECURITY_ATTRIBUTES *, int, int, HANDLE *);
BOOL WINAPI CreateProcessAsUserW(HANDLE, const WCHAR *, WCHAR *, SECURITY_ATTRIBUTES *, SECURITY_ATTRIBUTES *, BOOL, DWORD, void *, const WCHAR *, STARTUPINFOW *, PROCESS_INFORMATION *);

// msi.dll
UINT WINAPI MsiGetPropertyW(MSIHANDLE, const WCHAR *, WCHAR *, DWORD *);
MSIHANDLE WINAPI MsiCreateRecord(UINT);
UINT WINAPI MsiRecordSetStringW(MSIHANDLE, UINT, const WCHAR *);
int WINAPI MsiProcessMessage(MSIHANDLE, int, MSIHANDLE);
UINT WINAPI MsiCloseHandle(MSIHANDLE);

enum {
	ERROR_SUCCESS = 0,
	ERROR_MORE_DATA = 234,
	ERROR_NO_TOKEN = 1008,
	ERROR_INSTALL_USEREXIT = 1602,
	ERROR_INSTALL_FAILURE = 1603,

	HANDLE_FLAG_INHERIT = 1,
	STARTF_USESTDHANDLES = 0x100,
	CREATE_NO_WINDOW = 0x08000000,
	CREATE_UNICODE_ENVIRONMENT = 0x400,
	INFINITE = 0xFFFFFFFF,
	WAIT_OBJECT_0 = 0,

	TOKEN_ASSIGN_PRIMARY = 1,
	TOKEN_DUPLICATE = 2,
	TOKEN_QUERY = 8,
	MAXIMUM_ALLOWED = 0x02000000,
	SecurityImpersonation = 2,
	TokenPrimary = 1,

	HEAP_ZERO_MEMORY = 8,
	CP_OEMCP = 1,
	CP_UTF8 = 65001,
	MB_ERR_INVALID_CHARS = 8,

	INSTALLMESSAGE_ERROR = 0x01000000,
	INSTALLMESSAGE_INFO = 0x04000000,
	IDCANCEL = 2,
};

// The compiler may lower struct copies and zeroing to these.
void *memset(void *dst, int value, SIZE_T len)
{
	BYTE *p = dst;
	while (len--) {
		*p++ = (BYTE)value;
	}
	return dst;
}

void *memcpy(void *dst, const void *src, SIZE_T len)
{
	BYTE *d = dst;
	const BYTE *s = src;
	while (len--) {
		*d++ = *s++;
	}
	return dst;
}

static SIZE_T wlen(const WCHAR *s)
{
	SIZE_T n = 0;
	while (s[n]) {
		n++;
	}
	return n;
}

static void wappend(WCHAR *dst, SIZE_T *pos, const WCHAR *src)
{
	while (*src) {
		dst[(*pos)++] = *src++;
	}
	dst[*pos] = 0;
}

static void wappendnum(WCHAR *dst, SIZE_T *pos, DWORD value)
{
	WCHAR digits[12];
	int n = 0;
	do {
		digits[n++] = (WCHAR)('0' + value % 10);
		value /= 10;
	} while (value);
	while (n) {
		dst[(*pos)++] = digits[--n];
	}
	dst[*pos] = 0;
}

// logw writes one informational line to the installer log. The text goes into
// field 1 and the template is fixed, so brackets in the process output are
// not formatted as properties.
static int logw(MSIHANDLE install, const WCHAR *text)
{
	static const WCHAR template[] = {'G', 'o', 'M', 's', 'i', 'R', 'u', 'n', ':', ' ', '[', '1', ']', 0};
	MSIHANDLE rec = MsiCreateRecord(1);
	if (!rec) {
		return 0;
	}
	MsiRecordSetStringW(rec, 0, template);
	MsiRecordSetStringW(rec, 1, text);
	int ret = MsiProcessMessage(install, INSTALLMESSAGE_INFO, rec);
	MsiCloseHandle(rec);
	return ret;
}

// logf writes "<what>: <detail> (error <code>)" without the code when it is 0.
static void logf(MSIHANDLE install, const WCHAR *what, const WCHAR *detail, DWORD code)
{
	static const WCHAR sep[] = {':', ' ', 0};
	static const WCHAR errOpen[] = {' ', '(', 'e', 'r', 'r', 'o', 'r', ' ', 0};
	static const WCHAR errClose[] = {')', 0};
	SIZE_T n = wlen(what) + 2 + (detail ? wlen(detail) : 0) + 24;
	WCHAR *buf = HeapAlloc(GetProcessHeap(), 0, (n + 1) * sizeof(WCHAR));
	if (!buf) {
		return;
	}
	SIZE_T pos = 0;
	wappend(buf, &pos, what);
	if (detail) {
		wappend(buf, &pos, sep);
		wappend(buf, &pos, detail);
	}
	if (code) {
		wappend(buf, &pos, errOpen);
		wappendnum(buf, &pos, code);
		wappend(buf, &pos, errClose);
	}
	logw(install, buf);
	HeapFree(GetProcessHeap(), 0, buf);
}

// property reads a property into a heap buffer, or returns 0 when it is
// empty or unreadable.
static WCHAR *property(MSIHANDLE install, const WCHAR *name)
{
	WCHAR probe[1];
	DWORD size = 0;
	UINT err = MsiGetPropertyW(install, name, probe, &size);
	if (err != ERROR_MORE_DATA && err != ERROR_SUCCESS) {
		return 0;
	}
	if (size == 0) {
		return 0;
	}
	size++;
	WCHAR *buf = HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, size * sizeof(WCHAR));
	if (!buf) {
		return 0;
	}
	if (MsiGetPropertyW(install, name, buf, &size) != ERROR_SUCCESS || size == 0) {
		HeapFree(GetProcessHeap(), 0, buf);
		return 0;
	}
	return buf;
}

// emitLine converts one line of process output to UTF-16 and logs it. UTF-8
// is tried first, so Go programs and most modern tools log correctly; invalid
// UTF-8 is read in the OEM code page, which is what cmd.exe and PowerShell
// write to a redirected pipe.
static int emitLine(MSIHANDLE install, const char *line, int len)
{
	if (len == 0) {
		static const WCHAR empty[] = {0};
		return logw(install, empty);
	}
	int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, line, len, 0, 0);
	UINT cp = CP_UTF8;
	if (n == 0) {
		cp = CP_OEMCP;
		n = MultiByteToWideChar(CP_OEMCP, 0, line, len, 0, 0);
	}
	if (n == 0) {
		return 0;
	}
	WCHAR *buf = HeapAlloc(GetProcessHeap(), 0, ((SIZE_T)n + 1) * sizeof(WCHAR));
	if (!buf) {
		return 0;
	}
	n = MultiByteToWideChar(cp, 0, line, len, buf, n);
	buf[n] = 0;
	int ret = logw(install, buf);
	HeapFree(GetProcessHeap(), 0, buf);
	return ret;
}

// pump reads the pipe until the child closes it and logs every line. It
// returns IDCANCEL when the installer asked to cancel.
static int pump(MSIHANDLE install, HANDLE pipe)
{
	HANDLE heap = GetProcessHeap();
	SIZE_T cap = 4096, len = 0;
	char *buf = HeapAlloc(heap, 0, cap);
	int cancelled = 0;
	if (!buf) {
		return 0;
	}
	for (;;) {
		if (cap - len < 1024) {
			cap *= 2;
			char *grown = HeapReAlloc(heap, 0, buf, cap);
			if (!grown) {
				break;
			}
			buf = grown;
		}
		DWORD got = 0;
		if (!ReadFile(pipe, buf + len, (DWORD)(cap - len), &got, 0) || got == 0) {
			break;
		}
		len += got;
		SIZE_T start = 0;
		for (SIZE_T i = 0; i < len; i++) {
			if (buf[i] == '\n') {
				SIZE_T end = i;
				if (end > start && buf[end - 1] == '\r') {
					end--;
				}
				if (emitLine(install, buf + start, (int)(end - start)) == IDCANCEL) {
					cancelled = 1;
				}
				start = i + 1;
			}
		}
		if (start) {
			memcpy(buf, buf + start, len - start);
			len -= start;
		}
	}
	if (len) {
		if (emitLine(install, buf, (int)len) == IDCANCEL) {
			cancelled = 1;
		}
	}
	HeapFree(heap, 0, buf);
	return cancelled ? IDCANCEL : 0;
}

static const WCHAR msgNoData[] = {'n', 'o', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgBadData[] = {'m', 'a', 'l', 'f', 'o', 'r', 'm', 'e', 'd', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgPipe[] = {'C', 'r', 'e', 'a', 't', 'e', 'P', 'i', 'p', 'e', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgStart[] = {'s', 't', 'a', 'r', 't', 'i', 'n', 'g', ' ', 'i', 'n', 0};
static const WCHAR msgDefaultDir[] = {'(', 'c', 'u', 'r', 'r', 'e', 'n', 't', ' ', 'd', 'i', 'r', 'e', 'c', 't', 'o', 'r', 'y', ')', 0};
static const WCHAR msgAsUser[] = {'a', 's', ' ', 't', 'h', 'e', ' ', 'i', 'm', 'p', 'e', 'r', 's', 'o', 'n', 'a', 't', 'e', 'd', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR msgAsProcess[] = {'w', 'i', 't', 'h', ' ', 't', 'h', 'e', ' ', 'p', 'r', 'o', 'c', 'e', 's', 's', ' ', 't', 'o', 'k', 'e', 'n', 0};
static const WCHAR msgCreate[] = {'C', 'r', 'e', 'a', 't', 'e', 'P', 'r', 'o', 'c', 'e', 's', 's', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgToken[] = {'D', 'u', 'p', 'l', 'i', 'c', 'a', 't', 'e', 'T', 'o', 'k', 'e', 'n', 'E', 'x', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgExit[] = {'e', 'x', 'i', 't', ' ', 'c', 'o', 'd', 'e', 0};
static const WCHAR msgCancel[] = {'c', 'a', 'n', 'c', 'e', 'l', 'l', 'e', 'd', ' ', 'b', 'y', ' ', 't', 'h', 'e', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR propData[] = {'C', 'u', 's', 't', 'o', 'm', 'A', 'c', 't', 'i', 'o', 'n', 'D', 'a', 't', 'a', 0};
static const WCHAR propRun[] = {'G', 'o', 'M', 's', 'i', 'R', 'u', 'n', 0};

// parse splits "<n>:<dir><command>" into dir (0 when n is 0) and command. The
// directory is terminated in place.
static int parse(WCHAR *data, WCHAR **dir, WCHAR **cmd)
{
	SIZE_T n = 0, i = 0;
	if (data[0] < '0' || data[0] > '9') {
		return 0;
	}
	while (data[i] >= '0' && data[i] <= '9') {
		n = n * 10 + (SIZE_T)(data[i] - '0');
		i++;
	}
	if (data[i] != ':') {
		return 0;
	}
	i++;
	if (wlen(data + i) < n + 1) {
		return 0;
	}
	*dir = n ? data + i : 0;
	// The command needs a buffer of its own: CreateProcessW may modify it.
	SIZE_T cmdLen = wlen(data + i + n);
	WCHAR *copy = HeapAlloc(GetProcessHeap(), 0, (cmdLen + 1) * sizeof(WCHAR));
	if (!copy) {
		return 0;
	}
	memcpy(copy, data + i + n, (cmdLen + 1) * sizeof(WCHAR));
	*cmd = copy;
	data[i + n] = 0;
	return 1;
}

// start creates the child with the impersonated user's token when the thread
// impersonates one (an impersonated deferred action), and with the process
// token otherwise. CreateProcessW alone would ignore the impersonation token.
static BOOL start(MSIHANDLE install, WCHAR *cmd, const WCHAR *dir, STARTUPINFOW *si, PROCESS_INFORMATION *pi)
{
	HANDLE thread = 0, primary = 0;
	BOOL ok;
	if (OpenThreadToken(GetCurrentThread(), TOKEN_QUERY | TOKEN_DUPLICATE | TOKEN_ASSIGN_PRIMARY, 1, &thread)) {
		if (!DuplicateTokenEx(thread, MAXIMUM_ALLOWED, 0, SecurityImpersonation, TokenPrimary, &primary)) {
			logf(install, msgToken, 0, GetLastError());
			CloseHandle(thread);
			return 0;
		}
		logf(install, msgStart, msgAsUser, 0);
		ok = CreateProcessAsUserW(primary, 0, cmd, 0, 0, 1, CREATE_NO_WINDOW, 0, dir, si, pi);
		DWORD err = GetLastError();
		CloseHandle(primary);
		CloseHandle(thread);
		if (!ok) {
			logf(install, msgCreate, 0, err);
		}
		return ok;
	}
	logf(install, msgStart, msgAsProcess, GetLastError() == ERROR_NO_TOKEN ? 0 : GetLastError());
	ok = CreateProcessW(0, cmd, 0, 0, 1, CREATE_NO_WINDOW, 0, dir, si, pi);
	if (!ok) {
		logf(install, msgCreate, 0, GetLastError());
	}
	return ok;
}

__declspec(dllexport) UINT WINAPI GoMsiRun(MSIHANDLE install)
{
	WCHAR *data = property(install, propData);
	if (!data) {
		data = property(install, propRun);
	}
	if (!data) {
		logf(install, msgNoData, 0, 0);
		return ERROR_INSTALL_FAILURE;
	}
	WCHAR *dir = 0, *cmd = 0;
	if (!parse(data, &dir, &cmd)) {
		logf(install, msgBadData, 0, 0);
		HeapFree(GetProcessHeap(), 0, data);
		return ERROR_INSTALL_FAILURE;
	}
	logf(install, msgStart, dir ? dir : msgDefaultDir, 0);

	SECURITY_ATTRIBUTES sa = {sizeof sa, 0, 1};
	HANDLE readEnd = 0, writeEnd = 0;
	UINT result = ERROR_INSTALL_FAILURE;
	if (!CreatePipe(&readEnd, &writeEnd, &sa, 0)) {
		logf(install, msgPipe, 0, GetLastError());
		goto done;
	}
	SetHandleInformation(readEnd, HANDLE_FLAG_INHERIT, 0);

	STARTUPINFOW si;
	PROCESS_INFORMATION pi;
	memset(&si, 0, sizeof si);
	memset(&pi, 0, sizeof pi);
	si.cb = sizeof si;
	si.dwFlags = STARTF_USESTDHANDLES;
	si.hStdOutput = writeEnd;
	si.hStdError = writeEnd;
	si.hStdInput = 0;
	if (!start(install, cmd, dir, &si, &pi)) {
		CloseHandle(writeEnd);
		CloseHandle(readEnd);
		goto done;
	}
	CloseHandle(writeEnd);
	CloseHandle(pi.hThread);
	int cancelled = pump(install, readEnd);
	CloseHandle(readEnd);
	if (cancelled == IDCANCEL) {
		TerminateProcess(pi.hProcess, 1);
	}
	WaitForSingleObject(pi.hProcess, INFINITE);
	DWORD exitCode = 1;
	GetExitCodeProcess(pi.hProcess, &exitCode);
	CloseHandle(pi.hProcess);
	{
		WCHAR line[32];
		SIZE_T pos = 0;
		static const WCHAR sp[] = {' ', 0};
		wappend(line, &pos, msgExit);
		wappend(line, &pos, sp);
		wappendnum(line, &pos, exitCode);
		logw(install, line);
	}
	if (cancelled == IDCANCEL) {
		logf(install, msgCancel, 0, 0);
		result = ERROR_INSTALL_USEREXIT;
	} else {
		result = exitCode == 0 ? ERROR_SUCCESS : ERROR_INSTALL_FAILURE;
	}
done:
	HeapFree(GetProcessHeap(), 0, cmd);
	HeapFree(GetProcessHeap(), 0, data);
	return result;
}

BOOL WINAPI _DllMainCRTStartup(HANDLE instance, DWORD reason, void *reserved)
{
	(void)instance;
	(void)reason;
	(void)reserved;
	return 1;
}
