// builderrun.c is the custom action DLL that msi-builder embeds into packages for
// `run` actions. The entry point BuilderRun starts a command line with a
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
// the property BuilderRun for immediate actions:
//
//     <working directory>|<command line>
//
// The directory may be empty; `|` cannot occur in a Windows path, so the
// first one ends it. A log line reads `BuilderRun: ...`.

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

// ACL, ACE_HEADER and ACCESS_ALLOWED_ACE as winnt.h documents them.
typedef struct {
    BYTE AclRevision;
    BYTE Sbz1;
    unsigned short AclSize;
    unsigned short AceCount;
    unsigned short Sbz2;
} ACL;

typedef struct {
    BYTE AceType;
    BYTE AceFlags;
    unsigned short AceSize;
} ACE_HEADER;

typedef struct {
    ACE_HEADER Header;
    DWORD Mask;
    DWORD SidStart;
} ACCESS_ALLOWED_ACE;

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
void *WINAPI LocalFree(void *);
BOOL WINAPI GetComputerNameW(WCHAR *, DWORD *);

// advapi32.dll
BOOL WINAPI OpenThreadToken(HANDLE, DWORD, BOOL, HANDLE *);
BOOL WINAPI DuplicateTokenEx(HANDLE, DWORD, SECURITY_ATTRIBUTES *, int, int, HANDLE *);
BOOL WINAPI CreateProcessAsUserW(HANDLE, const WCHAR *, WCHAR *, SECURITY_ATTRIBUTES *, SECURITY_ATTRIBUTES *, BOOL, DWORD, void *, const WCHAR *, STARTUPINFOW *, PROCESS_INFORMATION *);

BOOL WINAPI ConvertStringSidToSidW(const WCHAR *, void **);
BOOL WINAPI LookupAccountNameW(const WCHAR *, const WCHAR *, void *, DWORD *, WCHAR *, DWORD *, int *);
DWORD WINAPI GetNamedSecurityInfoW(const WCHAR *, int, DWORD, void **, void **, ACL **, ACL **, void **);
DWORD WINAPI SetNamedSecurityInfoW(WCHAR *, int, DWORD, void *, void *, ACL *, ACL *);
BOOL WINAPI GetSecurityDescriptorControl(void *, unsigned short *, DWORD *);
BOOL WINAPI InitializeAcl(ACL *, DWORD, DWORD);
BOOL WINAPI GetAce(ACL *, DWORD, void **);
BOOL WINAPI AddAce(ACL *, DWORD, DWORD, void *, DWORD);
BOOL WINAPI AddAccessAllowedAceEx(ACL *, DWORD, DWORD, DWORD, void *);
BOOL WINAPI EqualSid(void *, void *);
DWORD WINAPI GetLengthSid(void *);
long WINAPI RegCreateKeyExW(HANDLE, const WCHAR *, DWORD, WCHAR *, DWORD, DWORD, SECURITY_ATTRIBUTES *, HANDLE *, DWORD *);
long WINAPI RegOpenKeyExW(HANDLE, const WCHAR *, DWORD, DWORD, HANDLE *);
long WINAPI RegSetValueExW(HANDLE, const WCHAR *, DWORD, DWORD, const BYTE *, DWORD);
long WINAPI RegQueryValueExW(HANDLE, const WCHAR *, DWORD *, DWORD *, BYTE *, DWORD *);
long WINAPI RegDeleteTreeW(HANDLE, const WCHAR *);
long WINAPI RegCloseKey(HANDLE);

// msi.dll
UINT WINAPI MsiGetPropertyW(MSIHANDLE, const WCHAR *, WCHAR *, DWORD *);
MSIHANDLE WINAPI MsiCreateRecord(UINT);
UINT WINAPI MsiRecordSetStringW(MSIHANDLE, UINT, const WCHAR *);
int WINAPI MsiProcessMessage(MSIHANDLE, int, MSIHANDLE);
UINT WINAPI MsiCloseHandle(MSIHANDLE);
UINT WINAPI MsiSetPropertyW(MSIHANDLE, const WCHAR *, const WCHAR *);
UINT WINAPI MsiFormatRecordW(MSIHANDLE, MSIHANDLE, WCHAR *, DWORD *);
int WINAPI MsiEvaluateConditionW(MSIHANDLE, const WCHAR *);

enum {
    ERROR_SUCCESS = 0,
    ERROR_MORE_DATA = 234,
    ERROR_NO_TOKEN = 1008,
    ERROR_INSTALL_USEREXIT = 1602,
    ERROR_INSTALL_FAILURE = 1603,
    ERROR_INSUFFICIENT_BUFFER = 122,

    HANDLE_FLAG_INHERIT = 1,
    STARTF_USESTDHANDLES = 0x100,
    CREATE_NO_WINDOW = 0x08000000,
    INFINITE = 0xFFFFFFFF,
    WAIT_OBJECT_0 = 0,

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

    SE_FILE_OBJECT = 1,
    SE_SERVICE = 2,
    DACL_SECURITY_INFORMATION = 4,
    PROTECTED_DACL_SECURITY_INFORMATION = 0x80000000,
    UNPROTECTED_DACL_SECURITY_INFORMATION = 0x20000000,
    SE_DACL_PROTECTED = 0x1000,
    ACL_REVISION = 2,
    ACCESS_ALLOWED_ACE_TYPE = 0,
    INHERITED_ACE = 0x10,
    VALID_INHERIT_FLAGS = 0x1F,
    MAXDWORD_ = 0xFFFFFFFF,

    KEY_QUERY_VALUE = 0x1,
    KEY_SET_VALUE = 0x2,
    REG_DWORD = 4,
    MSICONDITION_TRUE = 1,
    MSICONDITION_NONE = 2,
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

// Log templates: the text goes into field 1 and the template is fixed, so
// brackets in process output or object names are not formatted as
// properties.
static const WCHAR tmplRun[] = {'B', 'u', 'i', 'l', 'd', 'e', 'r', 'R', 'u', 'n', ':', ' ', '[', '1', ']', 0};
static const WCHAR tmplGrant[] = {'B', 'u', 'i', 'l', 'd', 'e', 'r', 'G', 'r', 'a', 'n', 't', ':', ' ', '[', '1', ']', 0};

// logt writes one informational line to the installer log.
static int logt(MSIHANDLE install, const WCHAR *template, const WCHAR *text)
{
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

static int logw(MSIHANDLE install, const WCHAR *text)
{
    return logt(install, tmplRun, text);
}

// logft writes "<what>: <detail> (error <code>)" without the code when it is 0.
static void logft(MSIHANDLE install, const WCHAR *template, const WCHAR *what, const WCHAR *detail, DWORD code)
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
    logt(install, template, buf);
    HeapFree(GetProcessHeap(), 0, buf);
}

static void logf(MSIHANDLE install, const WCHAR *what, const WCHAR *detail, DWORD code)
{
    logft(install, tmplRun, what, detail, code);
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

// pump reads the pipe until the child closes it and logs every line. When
// the installer asks to cancel, the child is terminated and the pipe is
// drained until it closes (a grandchild may still hold the write end); the
// result is then IDCANCEL.
static int pump(MSIHANDLE install, HANDLE pipe, HANDLE process)
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
        SIZE_T scanFrom = len;
        len += got;
        SIZE_T start = 0;
        for (SIZE_T i = scanFrom; i < len; i++) {
            if (buf[i] == '\n') {
                SIZE_T end = i;
                if (end > start && buf[end - 1] == '\r') {
                    end--;
                }
                if (emitLine(install, buf + start, (int)(end - start)) == IDCANCEL && !cancelled) {
                    cancelled = 1;
                    TerminateProcess(process, 1);
                }
                start = i + 1;
            }
        }
        if (start) {
            memcpy(buf, buf + start, len - start);
            len -= start;
        }
    }
    if (len && emitLine(install, buf, (int)len) == IDCANCEL && !cancelled) {
        cancelled = 1;
        TerminateProcess(process, 1);
    }
    HeapFree(heap, 0, buf);
    return cancelled ? IDCANCEL : 0;
}

static const WCHAR msgNoData[] = {'n', 'o', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgBadData[] = {'m', 'a', 'l', 'f', 'o', 'r', 'm', 'e', 'd', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgPipe[] = {'C', 'r', 'e', 'a', 't', 'e', 'P', 'i', 'p', 'e', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgStart[] = {'s', 't', 'a', 'r', 't', 'i', 'n', 'g', ' ', 'i', 'n', 0};
static const WCHAR msgDefaultDir[] = {'(', 'c', 'u', 'r', 'r', 'e', 'n', 't', ' ', 'd', 'i', 'r', 'e', 'c', 't', 'o', 'r', 'y', ')', 0};
static const WCHAR msgToken[] = {'t', 'o', 'k', 'e', 'n', 0};
static const WCHAR msgAsUser[] = {'t', 'h', 'e', ' ', 'i', 'm', 'p', 'e', 'r', 's', 'o', 'n', 'a', 't', 'e', 'd', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR msgAsProcess[] = {'t', 'h', 'e', ' ', 'p', 'r', 'o', 'c', 'e', 's', 's', 0};
static const WCHAR msgCreate[] = {'C', 'r', 'e', 'a', 't', 'e', 'P', 'r', 'o', 'c', 'e', 's', 's', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgDupToken[] = {'D', 'u', 'p', 'l', 'i', 'c', 'a', 't', 'e', 'T', 'o', 'k', 'e', 'n', 'E', 'x', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgExit[] = {'e', 'x', 'i', 't', ' ', 'c', 'o', 'd', 'e', 0};
static const WCHAR msgCancel[] = {'c', 'a', 'n', 'c', 'e', 'l', 'l', 'e', 'd', ' ', 'b', 'y', ' ', 't', 'h', 'e', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR propData[] = {'C', 'u', 's', 't', 'o', 'm', 'A', 'c', 't', 'i', 'o', 'n', 'D', 'a', 't', 'a', 0};
static const WCHAR propRun[] = {'B', 'u', 'i', 'l', 'd', 'e', 'r', 'R', 'u', 'n', 0};

// parse splits "<dir>|<command>" into dir (0 when empty) and command. `|`
// cannot occur in a Windows path, so the first one ends the directory, which
// is terminated in place.
static int parse(WCHAR *data, WCHAR **dir, WCHAR **cmd)
{
    SIZE_T i = 0;
    while (data[i] && data[i] != '|') {
        i++;
    }
    if (data[i] != '|') {
        return 0;
    }
    *dir = i ? data : 0;
    // The command needs a buffer of its own: CreateProcessW may modify it.
    SIZE_T cmdLen = wlen(data + i + 1);
    WCHAR *copy = HeapAlloc(GetProcessHeap(), 0, (cmdLen + 1) * sizeof(WCHAR));
    if (!copy) {
        return 0;
    }
    memcpy(copy, data + i + 1, (cmdLen + 1) * sizeof(WCHAR));
    *cmd = copy;
    data[i] = 0;
    return 1;
}

// start creates the child with the impersonated user's token when the thread
// impersonates one (an impersonated deferred action), and with the process
// token otherwise. CreateProcessW alone would ignore the impersonation token.
static BOOL start(MSIHANDLE install, WCHAR *cmd, const WCHAR *dir, STARTUPINFOW *si, PROCESS_INFORMATION *pi)
{
    HANDLE thread = 0, primary = 0;
    BOOL ok;
    if (OpenThreadToken(GetCurrentThread(), TOKEN_QUERY | TOKEN_DUPLICATE, 1, &thread)) {
        if (!DuplicateTokenEx(thread, MAXIMUM_ALLOWED, 0, SecurityImpersonation, TokenPrimary, &primary)) {
            logf(install, msgDupToken, 0, GetLastError());
            CloseHandle(thread);
            return 0;
        }
        logf(install, msgToken, msgAsUser, 0);
        ok = CreateProcessAsUserW(primary, 0, cmd, 0, 0, 1, CREATE_NO_WINDOW, 0, dir, si, pi);
        DWORD err = GetLastError();
        CloseHandle(primary);
        CloseHandle(thread);
        if (!ok) {
            logf(install, msgCreate, 0, err);
        }
        return ok;
    }
    logf(install, msgToken, msgAsProcess, GetLastError() == ERROR_NO_TOKEN ? 0 : GetLastError());
    ok = CreateProcessW(0, cmd, 0, 0, 1, CREATE_NO_WINDOW, 0, dir, si, pi);
    if (!ok) {
        logf(install, msgCreate, 0, GetLastError());
    }
    return ok;
}

__declspec(dllexport) UINT WINAPI BuilderRun(MSIHANDLE install)
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
    int cancelled = pump(install, readEnd, pi.hProcess);
    CloseHandle(readEnd);
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

// Permission grants. BuilderGrantSchedule, an immediate action, reads the
// property BuilderGrants that the builder writes, one entry per line:
//
//     <condition> US <kind>|<mask>|<flags> US <account> US <object>
//
// (US is U+001F). For every entry whose condition is true (an empty one is),
// it formats account and object and hands the list to the deferred action
// BuilderGrantExec, its rollback BuilderGrantRollback and the commit action
// BuilderGrantCommit, all on the entry point BuilderGrant, as
// CustomActionData:
//
//     <op><ProductCode>
//     <kind>|<mask>|<flags>|<account>|<object>
//     ...
//
// op is `+` (grant), `-` (rollback) or `c` (commit). kind is `f` (file or
// folder) or `s` (service), mask and flags are hexadecimal access rights and
// ACE flags, the account is a SID string, an SDDL alias such as NS, a service
// manager name such as LocalSystem, or a name for LookupAccountNameW
// (`.\name` is a local account), and the object is a path or a service name.
//
// A grant adds the rights to the account's explicit allow ACE with the same
// inheritance flags, or adds such an ACE after the last explicit ACE, and
// keeps every other ACE, the inherited ones included. Before it changes an
// object it records the rights it adds, which the account did not have, in
// HKCU\Software\msi-builder\GrantRollback\<ProductCode> under the entry's
// number (as LocalSystem, HKCU is the LocalSystem hive). The rollback takes
// exactly those rights away again and the commit deletes the record, so rights
// that were there before the install survive a rollback. Log lines read
// `BuilderGrant: ...`.

static const WCHAR propGrants[] = u"BuilderGrants";
static const WCHAR propProductCode[] = u"ProductCode";
static const WCHAR propExec[] = u"BuilderGrantExec";
static const WCHAR propRollback[] = u"BuilderGrantRollback";
static const WCHAR propCommit[] = u"BuilderGrantCommit";
static const WCHAR rollbackKey[] = u"Software\\msi-builder\\GrantRollback\\";
static const WCHAR msgGrantData[] = u"malformed grant data";
static const WCHAR msgNoAccount[] = u"no account to grant rights to";
static const WCHAR msgBadAccount[] = u"the account contains | or a control character";
static const WCHAR msgBadObject[] = u"the object contains a control character";
static const WCHAR msgFormat[] = u"cannot format";
static const WCHAR msgCondition[] = u"cannot evaluate the condition";
static const WCHAR msgSid[] = u"cannot resolve the account";
static const WCHAR msgGetSec[] = u"cannot read the permissions of";
static const WCHAR msgSetSec[] = u"cannot write the permissions of";
static const WCHAR msgNoDacl[] = u"has no DACL, so everyone has full access; left as it is";
static const WCHAR msgAlready[] = u"already granted, left as it is";
static const WCHAR msgGranted[] = u"granted";
static const WCHAR msgRevoked[] = u"revoked";
static const WCHAR msgTo[] = u"to";
static const WCHAR msgAcl[] = u"cannot build the new DACL of";
static const WCHAR msgRecord[] = u"cannot record the grant for the rollback";
static const WCHAR msgNoMemory[] = u"out of memory";

// The service control manager's names of the built-in service accounts,
// which ServiceInstall.StartName uses: an account in a property that also
// names a service account comes in this form. LookupAccountNameW knows
// these accounts by other, localized names.
static const WCHAR *const serviceAccounts[][2] = {
    {u"LocalSystem", u"S-1-5-18"},
    {u".\\LocalSystem", u"S-1-5-18"},
    {u"NT AUTHORITY\\LocalService", u"S-1-5-19"},
    {u"NT AUTHORITY\\NetworkService", u"S-1-5-20"},
};

#if defined(__i386__)
typedef int INTPTR;
#else
typedef long long INTPTR;
#endif

// HKEY_CURRENT_USER, which winreg.h defines as the sign-extended 0x80000001.
#define HKCU_ ((HANDLE)(INTPTR)(int)0x80000001)

static WCHAR wlower(WCHAR c)
{
    return c >= 'A' && c <= 'Z' ? (WCHAR)(c - 'A' + 'a') : c;
}

static int wieq(const WCHAR *a, const WCHAR *b)
{
    while (*a && wlower(*a) == wlower(*b)) {
        a++;
        b++;
    }
    return *a == *b;
}

// wbuf is a growing heap string.
typedef struct {
    WCHAR *s;
    SIZE_T len, cap;
} wbuf;

static int wbufAdd(wbuf *b, const WCHAR *src, SIZE_T n)
{
    if (b->len + n + 1 > b->cap) {
        SIZE_T cap = (b->len + n + 1) * 2;
        WCHAR *grown = b->s ? HeapReAlloc(GetProcessHeap(), 0, b->s, cap * sizeof(WCHAR)) : HeapAlloc(GetProcessHeap(), 0, cap * sizeof(WCHAR));
        if (!grown) {
            return 0;
        }
        b->s = grown;
        b->cap = cap;
    }
    memcpy(b->s + b->len, src, n * sizeof(WCHAR));
    b->len += n;
    b->s[b->len] = 0;
    return 1;
}

// format formats a template with MsiFormatRecordW into a heap string.
static WCHAR *format(MSIHANDLE install, const WCHAR *template)
{
    MSIHANDLE rec = MsiCreateRecord(0);
    if (!rec) {
        return 0;
    }
    WCHAR *out = 0;
    WCHAR probe[1];
    DWORD size = 0;
    MsiRecordSetStringW(rec, 0, template);
    UINT err = MsiFormatRecordW(install, rec, probe, &size);
    if (err == ERROR_MORE_DATA || err == ERROR_SUCCESS) {
        size++;
        out = HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, (SIZE_T)size * sizeof(WCHAR));
        if (out && MsiFormatRecordW(install, rec, out, &size) != ERROR_SUCCESS) {
            HeapFree(GetProcessHeap(), 0, out);
            out = 0;
        }
    }
    MsiCloseHandle(rec);
    return out;
}

// trim removes leading and trailing spaces in place.
static WCHAR *trim(WCHAR *s)
{
    while (*s == ' ') {
        s++;
    }
    SIZE_T n = wlen(s);
    while (n && s[n - 1] == ' ') {
        s[--n] = 0;
    }
    return s;
}

static int hasControl(const WCHAR *s, int pipe)
{
    for (; *s; s++) {
        if (*s < 0x20 || (pipe && *s == '|')) {
            return 1;
        }
    }
    return 0;
}

// field cuts s at the next separator and returns the text after it, or 0.
static WCHAR *field(WCHAR *s, WCHAR sep)
{
    while (*s && *s != sep) {
        s++;
    }
    if (*s != sep) {
        return 0;
    }
    *s = 0;
    return s + 1;
}

// scheduleEntry formats one line of BuilderGrants and appends it to out. It
// returns 0 when the install has to fail.
static int scheduleEntry(MSIHANDLE install, WCHAR *line, wbuf *out)
{
    static const WCHAR nl[] = u"\n", bar[] = u"|";
    WCHAR *spec = field(line, 0x1F);
    WCHAR *account = spec ? field(spec, 0x1F) : 0;
    WCHAR *object = account ? field(account, 0x1F) : 0;
    if (!object) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        return 0;
    }
    if (*line) {
        int cond = MsiEvaluateConditionW(install, line);
        if (cond != MSICONDITION_TRUE && cond != MSICONDITION_NONE) {
            if (cond == 0) {
                return 1;
            }
            logft(install, tmplGrant, msgCondition, line, 0);
            return 0;
        }
    }
    WCHAR *acct = format(install, account), *obj = format(install, object);
    int ok = 0;
    if (!acct || !obj) {
        logft(install, tmplGrant, msgFormat, acct ? object : account, 0);
    } else {
        WCHAR *name = trim(acct);
        if (!*name) {
            logft(install, tmplGrant, msgNoAccount, obj, 0);
        } else if (hasControl(name, 1)) {
            logft(install, tmplGrant, msgBadAccount, name, 0);
        } else if (hasControl(obj, 0) || !*obj) {
            logft(install, tmplGrant, msgBadObject, obj, 0);
        } else {
            ok = wbufAdd(out, nl, 1) && wbufAdd(out, spec, wlen(spec)) && wbufAdd(out, bar, 1) && wbufAdd(out, name, wlen(name)) &&
                 wbufAdd(out, bar, 1) && wbufAdd(out, obj, wlen(obj));
            if (!ok) {
                logft(install, tmplGrant, msgNoMemory, 0, 0);
            }
        }
    }
    HeapFree(GetProcessHeap(), 0, acct);
    HeapFree(GetProcessHeap(), 0, obj);
    return ok;
}

__declspec(dllexport) UINT WINAPI BuilderGrantSchedule(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
    WCHAR *list = property(install, propGrants);
    WCHAR *product = property(install, propProductCode);
    wbuf out = {0, 0, 0};
    UINT result = ERROR_INSTALL_FAILURE;
    static const WCHAR plus[] = u"+";
    if (!product || !wbufAdd(&out, plus, 1) || !wbufAdd(&out, product, wlen(product))) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        goto done;
    }
    for (WCHAR *line = list; line && *line;) {
        WCHAR *next = field(line, '\n');
        if (!scheduleEntry(install, line, &out)) {
            goto done;
        }
        line = next;
    }
    result = ERROR_SUCCESS;
    const WCHAR *props[] = {propExec, propRollback, propCommit};
    const WCHAR ops[] = {'+', '-', 'c'};
    for (int i = 0; i < 3 && result == ERROR_SUCCESS; i++) {
        out.s[0] = ops[i];
        if (MsiSetPropertyW(install, props[i], out.s) != ERROR_SUCCESS) {
            result = ERROR_INSTALL_FAILURE;
        }
    }
done:
    HeapFree(heap, 0, out.s);
    HeapFree(heap, 0, product);
    HeapFree(heap, 0, list);
    return result;
}

// hexfield parses hexadecimal digits up to the next `|`, which it replaces
// with 0, and returns the text after it, or 0 when the field is malformed.
static WCHAR *hexfield(WCHAR *p, DWORD *value)
{
    DWORD v = 0;
    int digits = 0;
    for (; *p && *p != '|'; p++, digits++) {
        WCHAR c = wlower(*p);
        if (digits == 8) {
            return 0;
        }
        if (c >= '0' && c <= '9') {
            v = v << 4 | (DWORD)(c - '0');
        } else if (c >= 'a' && c <= 'f') {
            v = v << 4 | (DWORD)(c - 'a' + 10);
        } else {
            return 0;
        }
    }
    if (*p != '|' || digits == 0) {
        return 0;
    }
    *value = v;
    return p + 1;
}

// resolveSid returns the account's SID in a heap buffer.
static void *resolveSid(MSIHANDLE install, const WCHAR *account)
{
    HANDLE heap = GetProcessHeap();
    for (SIZE_T i = 0; i < sizeof serviceAccounts / sizeof serviceAccounts[0]; i++) {
        if (wieq(account, serviceAccounts[i][0])) {
            account = serviceAccounts[i][1];
            break;
        }
    }
    SIZE_T n = wlen(account);
    void *converted = 0;
    if (((n > 2 && account[0] == 'S' && account[1] == '-') || n == 2) && ConvertStringSidToSidW(account, &converted)) {
        DWORD size = GetLengthSid(converted);
        void *sid = HeapAlloc(heap, 0, size);
        if (sid) {
            memcpy(sid, converted, size);
        }
        LocalFree(converted);
        return sid;
    }
    // `.\name` is a local account: LookupAccountNameW takes it as
    // <computer>\name, while a bare name could also find a domain account.
    WCHAR *qualified = 0;
    const WCHAR *name = account;
    if (account[0] == '.' && account[1] == '\\') {
        WCHAR computer[64];
        DWORD len = sizeof computer / sizeof computer[0];
        if (!GetComputerNameW(computer, &len)) {
            logft(install, tmplGrant, msgSid, account, GetLastError());
            return 0;
        }
        qualified = HeapAlloc(heap, 0, ((SIZE_T)len + n + 1) * sizeof(WCHAR));
        if (!qualified) {
            return 0;
        }
        SIZE_T pos = 0;
        qualified[0] = 0;
        wappend(qualified, &pos, computer);
        wappend(qualified, &pos, account + 1);
        name = qualified;
    }
    DWORD sidSize = 0, domainSize = 0;
    int use = 0;
    void *sid = 0;
    WCHAR *domain = 0;
    LookupAccountNameW(0, name, 0, &sidSize, 0, &domainSize, &use);
    DWORD err = GetLastError();
    if (err != ERROR_INSUFFICIENT_BUFFER || !sidSize) {
        logft(install, tmplGrant, msgSid, account, err);
        goto done;
    }
    sid = HeapAlloc(heap, 0, sidSize);
    domain = HeapAlloc(heap, 0, ((SIZE_T)domainSize + 1) * sizeof(WCHAR));
    if (!sid || !domain || !LookupAccountNameW(0, name, sid, &sidSize, domain, &domainSize, &use)) {
        logft(install, tmplGrant, msgSid, account, GetLastError());
        HeapFree(heap, 0, sid);
        sid = 0;
    }
done:
    HeapFree(heap, 0, domain);
    HeapFree(heap, 0, qualified);
    return sid;
}

// rebuild copies dacl into *out with rights changed: on grant, mask is added
// to the account's explicit allow ACE with these flags, or a new ACE goes
// after the last explicit ACE, where the canonical order puts explicit allow
// ACEs, and *changed holds the rights the account did not have; on revoke,
// mask is taken from that ACE, which goes away once it has no rights left.
// *out stays 0 when nothing changes. It returns 0 on an error.
static int rebuild(ACL *dacl, void *sid, DWORD mask, DWORD flags, int grant, ACL **out, DWORD *changed)
{
    long found = -1;
    DWORD insertAt = dacl->AceCount, have = 0;
    *out = 0;
    *changed = 0;
    for (DWORD i = 0; i < dacl->AceCount; i++) {
        ACE_HEADER *ace;
        if (!GetAce(dacl, i, (void **)&ace)) {
            return 0;
        }
        if (ace->AceFlags & INHERITED_ACE) {
            if (insertAt == dacl->AceCount) {
                insertAt = i;
            }
            continue;
        }
        if (found < 0 && ace->AceType == ACCESS_ALLOWED_ACE_TYPE && (ace->AceFlags & VALID_INHERIT_FLAGS) == flags &&
            EqualSid(&((ACCESS_ALLOWED_ACE *)ace)->SidStart, sid)) {
            found = (long)i;
            have = ((ACCESS_ALLOWED_ACE *)ace)->Mask;
        }
    }
    *changed = grant ? mask & ~have : mask & have;
    if (!*changed) {
        return 1;
    }
    DWORD size = dacl->AclSize + (found < 0 ? sizeof(ACCESS_ALLOWED_ACE) + GetLengthSid(sid) : 0);
    DWORD revision = dacl->AclRevision > ACL_REVISION ? dacl->AclRevision : ACL_REVISION;
    ACL *acl = HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, size);
    if (!acl || !InitializeAcl(acl, size, revision)) {
        HeapFree(GetProcessHeap(), 0, acl);
        return 0;
    }
    int ok = 1;
    for (DWORD i = 0; ok && i <= dacl->AceCount; i++) {
        if (found < 0 && i == insertAt) {
            ok = AddAccessAllowedAceEx(acl, revision, flags, mask, sid);
        }
        if (!ok || i == dacl->AceCount) {
            break;
        }
        ACE_HEADER *ace;
        if (!GetAce(dacl, i, (void **)&ace)) {
            ok = 0;
            break;
        }
        if ((long)i != found) {
            ok = AddAce(acl, revision, MAXDWORD_, ace, ace->AceSize);
            continue;
        }
        DWORD rights = grant ? have | mask : have & ~mask;
        if (rights == 0) {
            continue;
        }
        ACCESS_ALLOWED_ACE *copy;
        ok = AddAce(acl, revision, MAXDWORD_, ace, ace->AceSize) && GetAce(acl, acl->AceCount - 1, (void **)&copy);
        if (ok) {
            copy->Mask = rights;
        }
    }
    if (!ok) {
        HeapFree(GetProcessHeap(), 0, acl);
        return 0;
    }
    *out = acl;
    return 1;
}

// applyEntry grants (or, on rollback, revokes) the rights of one entry. On a
// grant, record is called with the rights it is about to add before the
// object changes. It returns 0 on an error.
static int applyEntry(MSIHANDLE install, WCHAR *entry, int grant, HANDLE key, const WCHAR *valueName)
{
    HANDLE heap = GetProcessHeap();
    DWORD mask = 0, flags = 0;
    int type = entry[0] == 's' ? SE_SERVICE : SE_FILE_OBJECT;
    WCHAR *p = (entry[0] == 'f' || entry[0] == 's') && entry[1] == '|' ? hexfield(entry + 2, &mask) : 0;
    p = p ? hexfield(p, &flags) : 0;
    WCHAR *account = p, *object = p ? field(p, '|') : 0;
    if (!object || !*object || !*account || !mask || (flags & ~VALID_INHERIT_FLAGS)) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        return 0;
    }
    if (!grant) {
        // Only what this install added is taken away.
        DWORD added = 0, size = sizeof added, kind = 0;
        if (!key || RegQueryValueExW(key, valueName, 0, &kind, (BYTE *)&added, &size) || kind != REG_DWORD || !added) {
            return 1;
        }
        mask = added;
    }
    // A folder comes as [DIRECTORY], which ends in a backslash; a drive root
    // such as C:\ keeps it.
    SIZE_T objLen = wlen(object);
    if (objLen > 3 && object[objLen - 1] == '\\') {
        object[objLen - 1] = 0;
    }

    int ok = 0;
    void *sid = resolveSid(install, account);
    void *sd = 0;
    ACL *dacl = 0, *acl = 0;
    if (!sid) {
        goto done;
    }
    DWORD err = GetNamedSecurityInfoW(object, type, DACL_SECURITY_INFORMATION, 0, 0, &dacl, 0, &sd);
    if (err) {
        logft(install, tmplGrant, msgGetSec, object, err);
        goto done;
    }
    if (!dacl) {
        logft(install, tmplGrant, object, msgNoDacl, 0);
        ok = 1;
        goto done;
    }
    DWORD changed = 0;
    if (!rebuild(dacl, sid, mask, flags, grant, &acl, &changed)) {
        logft(install, tmplGrant, msgAcl, object, GetLastError());
        goto done;
    }
    if (!acl) {
        if (grant) {
            logft(install, tmplGrant, object, msgAlready, 0);
        }
        ok = 1;
        goto done;
    }
    if (grant && (!key || RegSetValueExW(key, valueName, 0, REG_DWORD, (const BYTE *)&changed, sizeof changed))) {
        logft(install, tmplGrant, msgRecord, object, 0);
        goto done;
    }
    // A file or folder keeps its protection state: a protected DACL stays
    // protected, and an unprotected one keeps inheriting from its parent.
    DWORD info = DACL_SECURITY_INFORMATION;
    if (type == SE_FILE_OBJECT) {
        unsigned short control = 0;
        DWORD revision = 0;
        if (!GetSecurityDescriptorControl(sd, &control, &revision)) {
            logft(install, tmplGrant, msgGetSec, object, GetLastError());
            goto done;
        }
        info |= control & SE_DACL_PROTECTED ? PROTECTED_DACL_SECURITY_INFORMATION : UNPROTECTED_DACL_SECURITY_INFORMATION;
    }
    err = SetNamedSecurityInfoW(object, type, info, 0, 0, acl, 0);
    if (err) {
        logft(install, tmplGrant, msgSetSec, object, err);
        goto done;
    }
    logft(install, tmplGrant, grant ? msgGranted : msgRevoked, object, 0);
    logft(install, tmplGrant, msgTo, account, 0);
    ok = 1;
done:
    if (sd) {
        LocalFree(sd);
    }
    HeapFree(heap, 0, acl);
    HeapFree(heap, 0, sid);
    return ok;
}

__declspec(dllexport) UINT WINAPI BuilderGrant(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
    WCHAR *data = property(install, propData);
    if (!data || (data[0] != '+' && data[0] != '-' && data[0] != 'c')) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        HeapFree(heap, 0, data);
        return data ? ERROR_INSTALL_FAILURE : ERROR_SUCCESS;
    }
    WCHAR op = data[0];
    WCHAR *entries = field(data + 1, '\n');
    wbuf path = {0, 0, 0};
    if (!wbufAdd(&path, rollbackKey, wlen(rollbackKey)) || !wbufAdd(&path, data + 1, wlen(data + 1))) {
        HeapFree(heap, 0, data);
        return op == '+' ? ERROR_INSTALL_FAILURE : ERROR_SUCCESS;
    }
    HANDLE key = 0;
    UINT result = ERROR_SUCCESS;
    if (op == '+') {
        // A record left by an install that never reached its commit is stale.
        RegDeleteTreeW(HKCU_, path.s);
        if (entries && RegCreateKeyExW(HKCU_, path.s, 0, 0, 0, KEY_QUERY_VALUE | KEY_SET_VALUE, 0, &key, 0)) {
            key = 0;
        }
    } else if (op == '-' && RegOpenKeyExW(HKCU_, path.s, 0, KEY_QUERY_VALUE, &key)) {
        key = 0;
    }
    DWORD index = 0;
    for (WCHAR *entry = entries; op != 'c' && entry && *entry; index++) {
        WCHAR *next = field(entry, '\n');
        WCHAR name[12];
        SIZE_T pos = 0;
        wappendnum(name, &pos, index);
        // A rollback undoes every entry it can and never fails.
        if (!applyEntry(install, entry, op == '+', key, name) && op == '+') {
            result = ERROR_INSTALL_FAILURE;
            break;
        }
        entry = next;
    }
    if (key) {
        RegCloseKey(key);
    }
    if (op != '+') {
        RegDeleteTreeW(HKCU_, path.s);
    }
    HeapFree(heap, 0, path.s);
    HeapFree(heap, 0, data);
    return result;
}

BOOL WINAPI _DllMainCRTStartup(HANDLE instance, DWORD reason, void *reserved)
{
    (void)instance;
    (void)reason;
    (void)reserved;
    return 1;
}
