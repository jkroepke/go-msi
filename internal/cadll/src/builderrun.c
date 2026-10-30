// builderrun.c is the custom action DLL that msi-builder embeds into packages for
// `run` actions. The entry point BuilderRun starts a command line with a
// hidden window, copies everything the process writes to stdout or stderr
// into the Windows Installer log through MsiProcessMessage, and turns the
// exit code into a custom action return value: 0 for exit code 0,
// ERROR_INSTALL_FAILURE for anything else.
//
// The DLL is freestanding: it includes no SDK header and links no C runtime,
// only kernel32.dll, advapi32.dll, msi.dll and user32.dll, whose prototypes are written
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
typedef int INTPTR;
#else
#define WINAPI
typedef SIZE_T_ SIZE_T;
typedef long long INTPTR;
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

// OVERLAPPED, STARTUPINFOEXW and the job limit structures as the SDK
// headers lay them out; the 64-bit members keep the 8-byte alignment that
// gives JOBOBJECT_EXTENDED_LIMIT_INFORMATION its documented size (112 bytes
// on x86, 144 on x64 and arm64).
typedef struct {
    SIZE_T Internal, InternalHigh;
    DWORD Offset, OffsetHigh;
    HANDLE hEvent;
} OVERLAPPED;

typedef struct {
    STARTUPINFOW StartupInfo;
    void *lpAttributeList;
} STARTUPINFOEXW;

typedef struct {
    long long PerProcessUserTimeLimit, PerJobUserTimeLimit;
    DWORD LimitFlags;
    SIZE_T MinimumWorkingSetSize, MaximumWorkingSetSize;
    DWORD ActiveProcessLimit;
    SIZE_T Affinity;
    DWORD PriorityClass, SchedulingClass;
} JOBOBJECT_BASIC_LIMIT_INFORMATION;

typedef struct {
    JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
    unsigned long long IoInfo[6];
    SIZE_T ProcessMemoryLimit, JobMemoryLimit, PeakProcessMemoryUsed, PeakJobMemoryUsed;
} JOBOBJECT_EXTENDED_LIMIT_INFORMATION;

_Static_assert(sizeof(OVERLAPPED) == (sizeof(void *) == 4 ? 20 : 32), "OVERLAPPED layout");
_Static_assert(sizeof(STARTUPINFOEXW) == (sizeof(void *) == 4 ? 72 : 112), "STARTUPINFOEXW layout");
_Static_assert(sizeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION) == (sizeof(void *) == 4 ? 112 : 144), "JOBOBJECT_EXTENDED_LIMIT_INFORMATION layout");

// kernel32.dll
HANDLE WINAPI CreateNamedPipeW(const WCHAR *, DWORD, DWORD, DWORD, DWORD, DWORD, DWORD, SECURITY_ATTRIBUTES *);
HANDLE WINAPI CreateFileW(const WCHAR *, DWORD, DWORD, SECURITY_ATTRIBUTES *, DWORD, DWORD, HANDLE);
HANDLE WINAPI CreateEventW(SECURITY_ATTRIBUTES *, BOOL, BOOL, const WCHAR *);
BOOL WINAPI CreateProcessW(const WCHAR *, WCHAR *, SECURITY_ATTRIBUTES *, SECURITY_ATTRIBUTES *, BOOL, DWORD, void *, const WCHAR *, STARTUPINFOW *, PROCESS_INFORMATION *);
BOOL WINAPI InitializeProcThreadAttributeList(void *, DWORD, DWORD, SIZE_T *);
BOOL WINAPI UpdateProcThreadAttribute(void *, DWORD, SIZE_T, void *, SIZE_T, void *, SIZE_T *);
void WINAPI DeleteProcThreadAttributeList(void *);
HANDLE WINAPI CreateJobObjectW(SECURITY_ATTRIBUTES *, const WCHAR *);
BOOL WINAPI SetInformationJobObject(HANDLE, int, void *, DWORD);
BOOL WINAPI AssignProcessToJobObject(HANDLE, HANDLE);
BOOL WINAPI TerminateJobObject(HANDLE, UINT);
DWORD WINAPI ResumeThread(HANDLE);
BOOL WINAPI TerminateProcess(HANDLE, UINT);
BOOL WINAPI ReadFile(HANDLE, void *, DWORD, DWORD *, OVERLAPPED *);
BOOL WINAPI GetOverlappedResult(HANDLE, OVERLAPPED *, DWORD *, BOOL);
BOOL WINAPI CancelIoEx(HANDLE, OVERLAPPED *);
DWORD WINAPI WaitForSingleObject(HANDLE, DWORD);
DWORD WINAPI WaitForMultipleObjects(DWORD, const HANDLE *, BOOL, DWORD);
unsigned long long WINAPI GetTickCount64(void);
BOOL WINAPI GetExitCodeProcess(HANDLE, DWORD *);
BOOL WINAPI CloseHandle(HANDLE);
HANDLE WINAPI GetCurrentThread(void);
HANDLE WINAPI GetProcessHeap(void);
void *WINAPI HeapAlloc(HANDLE, DWORD, SIZE_T);
void *WINAPI HeapReAlloc(HANDLE, DWORD, void *, SIZE_T);
BOOL WINAPI HeapFree(HANDLE, DWORD, void *);
DWORD WINAPI GetLastError(void);
int WINAPI MultiByteToWideChar(UINT, DWORD, const char *, int, WCHAR *, int);
void *WINAPI LocalFree(void *);
BOOL WINAPI GetComputerNameW(WCHAR *, DWORD *);

// advapi32.dll
BOOL WINAPI OpenThreadToken(HANDLE, DWORD, BOOL, HANDLE *);
BOOL WINAPI DuplicateTokenEx(HANDLE, DWORD, SECURITY_ATTRIBUTES *, int, int, HANDLE *);
BOOL WINAPI CreateProcessAsUserW(HANDLE, const WCHAR *, WCHAR *, SECURITY_ATTRIBUTES *, SECURITY_ATTRIBUTES *, BOOL, DWORD, void *, const WCHAR *, STARTUPINFOW *, PROCESS_INFORMATION *);
// RtlGenRandom, exported by advapi32.dll under this name.
BYTE WINAPI SystemFunction036(void *, DWORD); // returns BOOLEAN, one byte

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
UINT WINAPI MsiRecordSetInteger(MSIHANDLE, UINT, int);
int WINAPI MsiProcessMessage(MSIHANDLE, int, MSIHANDLE);
UINT WINAPI MsiCloseHandle(MSIHANDLE);
UINT WINAPI MsiSetPropertyW(MSIHANDLE, const WCHAR *, const WCHAR *);
UINT WINAPI MsiFormatRecordW(MSIHANDLE, MSIHANDLE, WCHAR *, DWORD *);
int WINAPI MsiEvaluateConditionW(MSIHANDLE, const WCHAR *);

// Values that do not fit an int are macros: C11 allows only int-sized
// enumerators (6.7.2.2), and clang accepts larger ones silently.
#define PROTECTED_DACL_SECURITY_INFORMATION 0x80000000u
#define MAXDWORD_ 0xFFFFFFFFu
#define INVALID_HANDLE_VALUE ((HANDLE)(INTPTR)-1)

enum {
    ERROR_SUCCESS = 0,
    ERROR_IO_PENDING = 997,
    ERROR_BROKEN_PIPE = 109,
    ERROR_MORE_DATA = 234,
    ERROR_NO_TOKEN = 1008,
    ERROR_INSTALL_USEREXIT = 1602,
    ERROR_INSTALL_FAILURE = 1603,
    ERROR_INSUFFICIENT_BUFFER = 122,

    STARTF_USESTDHANDLES = 0x100,
    CREATE_SUSPENDED = 4,
    CREATE_NO_WINDOW = 0x08000000,
    EXTENDED_STARTUPINFO_PRESENT = 0x00080000,
    PROC_THREAD_ATTRIBUTE_HANDLE_LIST = 0x20002,
    WAIT_OBJECT_0 = 0,
    WAIT_TIMEOUT = 258,

    PIPE_ACCESS_INBOUND = 1,
    FILE_FLAG_OVERLAPPED = 0x40000000,
    FILE_FLAG_FIRST_PIPE_INSTANCE = 0x00080000,
    PIPE_REJECT_REMOTE_CLIENTS = 8,
    GENERIC_WRITE = 0x40000000,
    OPEN_EXISTING = 3,

    JobObjectExtendedLimitInformation = 9,
    JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000,

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
    INSTALLMESSAGE_PROGRESS = 0x0A000000,
    IDCANCEL = 2,

    SE_FILE_OBJECT = 1,
    SE_SERVICE = 2,
    DACL_SECURITY_INFORMATION = 4,
    UNPROTECTED_DACL_SECURITY_INFORMATION = 0x20000000,
    SE_DACL_PROTECTED = 0x1000,
    ACL_REVISION = 2,
    ACCESS_ALLOWED_ACE_TYPE = 0,
    INHERITED_ACE = 0x10,
    VALID_INHERIT_FLAGS = 0x1F,
    // The inheritance flags a grant may author: INHERITED_ACE is the
    // system's mark, and an ACE that claims it is dropped at the next
    // propagation.
    AUTHORED_INHERIT_FLAGS = VALID_INHERIT_FLAGS & ~INHERITED_ACE,

    KEY_QUERY_VALUE = 0x1,
    KEY_SET_VALUE = 0x2,
    REG_SZ = 1,
    REG_DWORD = 4,
    MSICONDITION_TRUE = 1,
    MSICONDITION_NONE = 2,

    // SID_NAME_USE
    SidTypeDomain = 3,
    SidTypeDeletedAccount = 6,
    SidTypeInvalid = 7,
    SidTypeUnknown = 8,

    // Output handling: the line buffer, which is also the longest line
    // logged in one piece; how long the pipe is read after the child
    // exited while another process keeps it open; and how often a silent
    // child is interrupted to ask the installer about a cancel.
    LINE_BUFFER = 65536,
    EXIT_GRACE_MS = 5000,
    CANCEL_POLL_MS = 1000,
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

static void wappendhex(WCHAR *dst, SIZE_T *pos, BYTE value)
{
    static const char digits[] = "0123456789abcdef";
    dst[(*pos)++] = (WCHAR)digits[value >> 4];
    dst[(*pos)++] = (WCHAR)digits[value & 15];
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

// cancelled asks the installer whether the user cancelled: a progress
// message that adds nothing to the bar is not logged and returns IDCANCEL
// after a cancel, like any message from a custom action.
static int cancelled(MSIHANDLE install)
{
    MSIHANDLE rec = MsiCreateRecord(3);
    if (!rec) {
        return 0;
    }
    MsiRecordSetInteger(rec, 1, 2);
    MsiRecordSetInteger(rec, 2, 0);
    MsiRecordSetInteger(rec, 3, 0);
    int ret = MsiProcessMessage(install, INSTALLMESSAGE_PROGRESS, rec);
    MsiCloseHandle(rec);
    return ret == IDCANCEL;
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

static const WCHAR msgNoData[] = {'n', 'o', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgBadData[] = {'m', 'a', 'l', 'f', 'o', 'r', 'm', 'e', 'd', ' ', 'c', 'o', 'm', 'm', 'a', 'n', 'd', ' ', 'd', 'a', 't', 'a', 0};
static const WCHAR msgPipe[] = u"cannot create the output pipe";
static const WCHAR msgEvent[] = u"CreateEvent failed";
static const WCHAR msgAttributes[] = u"cannot build the handle list";
static const WCHAR msgJob[] = u"CreateJobObject failed; a process the command leaves behind is not ended";
static const WCHAR msgAssign[] = u"AssignProcessToJobObject failed; a process the command leaves behind is not ended";
static const WCHAR msgMemory[] = u"out of memory";
static const WCHAR msgStart[] = {'s', 't', 'a', 'r', 't', 'i', 'n', 'g', ' ', 'i', 'n', 0};
static const WCHAR msgDefaultDir[] = {'(', 'c', 'u', 'r', 'r', 'e', 'n', 't', ' ', 'd', 'i', 'r', 'e', 'c', 't', 'o', 'r', 'y', ')', 0};
static const WCHAR msgToken[] = {'t', 'o', 'k', 'e', 'n', 0};
static const WCHAR msgAsUser[] = {'t', 'h', 'e', ' ', 'i', 'm', 'p', 'e', 'r', 's', 'o', 'n', 'a', 't', 'e', 'd', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR msgAsProcess[] = {'t', 'h', 'e', ' ', 'p', 'r', 'o', 'c', 'e', 's', 's', 0};
static const WCHAR msgOpenToken[] = u"OpenThreadToken failed; not starting the command with the process token instead";
static const WCHAR msgCreate[] = {'C', 'r', 'e', 'a', 't', 'e', 'P', 'r', 'o', 'c', 'e', 's', 's', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgDupToken[] = {'D', 'u', 'p', 'l', 'i', 'c', 'a', 't', 'e', 'T', 'o', 'k', 'e', 'n', 'E', 'x', ' ', 'f', 'a', 'i', 'l', 'e', 'd', 0};
static const WCHAR msgExit[] = {'e', 'x', 'i', 't', ' ', 'c', 'o', 'd', 'e', 0};
static const WCHAR msgCancel[] = {'c', 'a', 'n', 'c', 'e', 'l', 'l', 'e', 'd', ' ', 'b', 'y', ' ', 't', 'h', 'e', ' ', 'u', 's', 'e', 'r', 0};
static const WCHAR msgAbandon[] = u"a process the command started keeps the output open after the command exited; its further output is not logged and it is ended";
static const WCHAR msgReadFail[] = u"reading the output failed; further output is not logged";
static const WCHAR msgResume[] = u"ResumeThread failed";
static const WCHAR propData[] = {'C', 'u', 's', 't', 'o', 'm', 'A', 'c', 't', 'i', 'o', 'n', 'D', 'a', 't', 'a', 0};
static const WCHAR propRun[] = {'B', 'u', 'i', 'l', 'd', 'e', 'r', 'R', 'u', 'n', 0};

// output is the state of the child's output: the pipe, its read event, the
// job that holds the child and what it starts, and the line buffer.
typedef struct {
    HANDLE pipe, event, process, job;
    char *buf;
    SIZE_T len;
    int skipLF; // the last byte was \r: a following \n ends no line
    int cancelled;
} output;

// stop ends the command: the job takes every process it started with it,
// the process alone when there is no job.
static void stop(output *o)
{
    if (o->job) {
        TerminateJobObject(o->job, 1);
    } else {
        TerminateProcess(o->process, 1);
    }
}

// emitLines logs the complete lines in the buffer, which the first n bytes
// of the last read extended, and keeps the rest. A line ends at \n, \r or
// \r\n, so progress written with \r alone is logged line by line; a line
// longer than the buffer is logged in pieces.
static void emitLines(MSIHANDLE install, output *o, SIZE_T scanFrom)
{
    SIZE_T start = 0;
    for (SIZE_T i = scanFrom; i < o->len; i++) {
        char c = o->buf[i];
        if (c == '\n' && o->skipLF && i == start) {
            o->skipLF = 0;
            start = i + 1;
            continue;
        }
        o->skipLF = 0;
        if (c != '\n' && c != '\r') {
            continue;
        }
        if (emitLine(install, o->buf + start, (int)(i - start)) == IDCANCEL && !o->cancelled) {
            o->cancelled = 1;
            stop(o);
        }
        o->skipLF = c == '\r';
        start = i + 1;
    }
    if (start) {
        memcpy(o->buf, o->buf + start, o->len - start);
        o->len -= start;
    }
    if (o->len == LINE_BUFFER) {
        if (emitLine(install, o->buf, LINE_BUFFER) == IDCANCEL && !o->cancelled) {
            o->cancelled = 1;
            stop(o);
        }
        o->len = 0;
    }
}

// abandon stops reading the pipe while the command may still run: the
// pipe is closed, so the child's writes fail instead of blocking on a full
// pipe, and the caller waits for the child without it.
static void abandon(output *o)
{
    CloseHandle(o->pipe);
    o->pipe = 0;
}

// pump reads the pipe until every writer has closed it and logs the lines.
// The read is overlapped, so the wait also watches the child and the
// installer: when the child has exited and another process keeps the pipe
// open past EXIT_GRACE_MS, that process is ended and the rest is not read;
// while the child is silent, the installer is asked every CANCEL_POLL_MS
// whether the user cancelled. A cancel, also one returned for a logged
// line, ends the job; the pipe then closes by itself.
static void pump(MSIHANDLE install, output *o)
{
    int exited = 0;
    unsigned long long deadline = 0, lastPoll = GetTickCount64();
    for (;;) {
        // The grace period is also checked here, so that a leftover process
        // that keeps writing does not keep the action alive.
        if (!exited && WaitForSingleObject(o->process, 0) == WAIT_OBJECT_0) {
            exited = 1;
            deadline = GetTickCount64() + EXIT_GRACE_MS;
        }
        if (exited && GetTickCount64() >= deadline) {
            logf(install, msgAbandon, 0, 0);
            stop(o);
            abandon(o);
            break;
        }
        OVERLAPPED ov;
        memset(&ov, 0, sizeof ov);
        ov.hEvent = o->event;
        DWORD got = 0;
        SIZE_T scanFrom = o->len;
        int eof = 0;
        if (!ReadFile(o->pipe, o->buf + o->len, (DWORD)(LINE_BUFFER - o->len), &got, &ov)) {
            DWORD err = GetLastError();
            if (err == ERROR_BROKEN_PIPE) {
                break;
            }
            if (err != ERROR_IO_PENDING) {
                logf(install, msgReadFail, 0, err);
                abandon(o);
                break;
            }
            int done = 0;
            while (!done) {
                HANDLE handles[2] = {o->event, o->process};
                DWORD r = WaitForMultipleObjects(exited ? 1 : 2, handles, 0, CANCEL_POLL_MS);
                if (r == WAIT_OBJECT_0) {
                    if (!GetOverlappedResult(o->pipe, &ov, &got, 0)) {
                        err = GetLastError();
                        if (err != ERROR_BROKEN_PIPE) {
                            logf(install, msgReadFail, 0, err);
                            abandon(o);
                        }
                        eof = 1;
                    }
                    done = 1;
                } else if (r == WAIT_OBJECT_0 + 1) {
                    exited = 1;
                    deadline = GetTickCount64() + EXIT_GRACE_MS;
                } else if (r != WAIT_TIMEOUT || (exited && GetTickCount64() >= deadline)) {
                    if (r == WAIT_TIMEOUT) {
                        logf(install, msgAbandon, 0, 0);
                    } else {
                        logf(install, msgReadFail, 0, GetLastError());
                    }
                    stop(o);
                    CancelIoEx(o->pipe, &ov);
                    GetOverlappedResult(o->pipe, &ov, &got, 1);
                    eof = 1;
                    done = 1;
                } else if (!exited && !o->cancelled) {
                    lastPoll = GetTickCount64();
                    if (cancelled(install)) {
                        o->cancelled = 1;
                        stop(o);
                    }
                }
            }
        }
        if (eof) {
            break;
        }
        // A zero-length write by the child reads as zero bytes; only a broken
        // pipe is the end.
        o->len += got;
        emitLines(install, o, scanFrom);
        // A child that writes without line ends completes no line and times
        // out no wait, so the poll runs on the clock here as well.
        if (!exited && !o->cancelled && GetTickCount64() - lastPoll >= CANCEL_POLL_MS) {
            lastPoll = GetTickCount64();
            if (cancelled(install)) {
                o->cancelled = 1;
                stop(o);
            }
        }
    }
    if (o->len && emitLine(install, o->buf, (int)o->len) == IDCANCEL && !o->cancelled) {
        o->cancelled = 1;
        stop(o);
    }
}

// await waits for the command to exit, asking the installer about a cancel
// while it runs.
static void await(MSIHANDLE install, output *o)
{
    while (WaitForSingleObject(o->process, CANCEL_POLL_MS) == WAIT_TIMEOUT) {
        if (!o->cancelled && cancelled(install)) {
            o->cancelled = 1;
            stop(o);
        }
    }
}

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

// makePipe creates the pipe the child writes to: a named pipe with a random
// name, because only a named pipe can be read overlapped. The server end is
// ours and not inheritable; the client end is the only inheritable handle
// the child gets. The random name, the single instance and the default
// security of a named pipe (full control for the creator, LocalSystem and
// administrators, read for everyone else) keep any other account from
// writing into the log.
static int makePipe(MSIHANDLE install, HANDLE *server, HANDLE *client)
{
    static const WCHAR prefix[] = u"\\\\.\\pipe\\BuilderRun-";
    BYTE random[16];
    if (!SystemFunction036(random, sizeof random)) {
        logf(install, msgPipe, 0, GetLastError());
        return 0;
    }
    WCHAR name[64];
    SIZE_T pos = 0;
    name[0] = 0;
    wappend(name, &pos, prefix);
    for (SIZE_T i = 0; i < sizeof random; i++) {
        wappendhex(name, &pos, random[i]);
    }
    SECURITY_ATTRIBUTES own = {sizeof own, 0, 0}, inherit = {sizeof inherit, 0, 1};
    *server = CreateNamedPipeW(name, PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED | FILE_FLAG_FIRST_PIPE_INSTANCE, PIPE_REJECT_REMOTE_CLIENTS, 1,
                               LINE_BUFFER, LINE_BUFFER, 0, &own);
    if (*server == INVALID_HANDLE_VALUE) {
        logf(install, msgPipe, 0, GetLastError());
        *server = 0;
        return 0;
    }
    *client = CreateFileW(name, GENERIC_WRITE, 0, &inherit, OPEN_EXISTING, 0, 0);
    if (*client == INVALID_HANDLE_VALUE) {
        logf(install, msgPipe, 0, GetLastError());
        CloseHandle(*server);
        *server = 0;
        *client = 0;
        return 0;
    }
    return 1;
}

// start creates the child, suspended, with the impersonated user's token
// when the thread impersonates one (an impersonated deferred action) and
// with the process token when it has none. CreateProcessW alone would
// ignore the impersonation token. Any other failure to read the thread's
// token fails the action rather than start the command as the process.
static BOOL start(MSIHANDLE install, WCHAR *cmd, const WCHAR *dir, STARTUPINFOEXW *si, PROCESS_INFORMATION *pi)
{
    const DWORD flags = CREATE_NO_WINDOW | CREATE_SUSPENDED | EXTENDED_STARTUPINFO_PRESENT;
    HANDLE thread = 0, primary = 0;
    BOOL ok;
    if (OpenThreadToken(GetCurrentThread(), TOKEN_QUERY | TOKEN_DUPLICATE, 1, &thread)) {
        if (!DuplicateTokenEx(thread, MAXIMUM_ALLOWED, 0, SecurityImpersonation, TokenPrimary, &primary)) {
            logf(install, msgDupToken, 0, GetLastError());
            CloseHandle(thread);
            return 0;
        }
        logf(install, msgToken, msgAsUser, 0);
        ok = CreateProcessAsUserW(primary, 0, cmd, 0, 0, 1, flags, 0, dir, &si->StartupInfo, pi);
        DWORD err = GetLastError();
        CloseHandle(primary);
        CloseHandle(thread);
        if (!ok) {
            logf(install, msgCreate, 0, err);
        }
        return ok;
    }
    DWORD err = GetLastError();
    if (err != ERROR_NO_TOKEN) {
        logf(install, msgOpenToken, 0, err);
        return 0;
    }
    logf(install, msgToken, msgAsProcess, 0);
    ok = CreateProcessW(0, cmd, 0, 0, 1, flags, 0, dir, &si->StartupInfo, pi);
    if (!ok) {
        logf(install, msgCreate, 0, GetLastError());
    }
    return ok;
}

// makeJob creates the job that holds the child and every process it
// starts, so that a cancel and the end of the action end them all; without
// it the action goes on with the child alone.
static HANDLE makeJob(MSIHANDLE install)
{
    HANDLE job = CreateJobObjectW(0, 0);
    if (!job) {
        logf(install, msgJob, 0, GetLastError());
        return 0;
    }
    JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits;
    memset(&limits, 0, sizeof limits);
    limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if (!SetInformationJobObject(job, JobObjectExtendedLimitInformation, &limits, sizeof limits)) {
        logf(install, msgJob, 0, GetLastError());
        CloseHandle(job);
        return 0;
    }
    return job;
}

__declspec(dllexport) UINT WINAPI BuilderRun(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
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
        HeapFree(heap, 0, data);
        return ERROR_INSTALL_FAILURE;
    }
    logf(install, msgStart, dir ? dir : msgDefaultDir, 0);

    UINT result = ERROR_INSTALL_FAILURE;
    output o;
    memset(&o, 0, sizeof o);
    HANDLE writeEnd = 0;
    void *attributes = 0;
    STARTUPINFOEXW si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof si);
    memset(&pi, 0, sizeof pi);

    o.buf = HeapAlloc(heap, 0, LINE_BUFFER);
    if (!o.buf) {
        logf(install, msgMemory, 0, 0);
        goto done;
    }
    if (!makePipe(install, &o.pipe, &writeEnd)) {
        goto done;
    }
    o.event = CreateEventW(0, 1, 0, 0);
    if (!o.event) {
        logf(install, msgEvent, 0, GetLastError());
        goto done;
    }
    {
        // The handle list limits what the child inherits to the pipe:
        // without it, every inheritable handle of the custom action server
        // goes along, including the pipe of another run action in flight.
        SIZE_T size = 0;
        InitializeProcThreadAttributeList(0, 1, 0, &size);
        attributes = HeapAlloc(heap, 0, size);
        if (!attributes || !InitializeProcThreadAttributeList(attributes, 1, 0, &size)) {
            logf(install, msgAttributes, 0, GetLastError());
            HeapFree(heap, 0, attributes);
            attributes = 0;
            goto done;
        }
        if (!UpdateProcThreadAttribute(attributes, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, &writeEnd, sizeof writeEnd, 0, 0)) {
            logf(install, msgAttributes, 0, GetLastError());
            goto done;
        }
    }
    si.StartupInfo.cb = sizeof si;
    si.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    si.StartupInfo.hStdOutput = writeEnd;
    si.StartupInfo.hStdError = writeEnd;
    si.StartupInfo.hStdInput = 0;
    si.lpAttributeList = attributes;
    o.job = makeJob(install);
    if (!start(install, cmd, dir, &si, &pi)) {
        goto done;
    }
    CloseHandle(writeEnd);
    writeEnd = 0;
    o.process = pi.hProcess;
    if (o.job && !AssignProcessToJobObject(o.job, pi.hProcess)) {
        logf(install, msgAssign, 0, GetLastError());
        CloseHandle(o.job);
        o.job = 0;
    }
    DWORD suspended = ResumeThread(pi.hThread);
    CloseHandle(pi.hThread);
    if (suspended == MAXDWORD_) {
        logf(install, msgResume, 0, GetLastError());
        stop(&o);
        goto done;
    }
    pump(install, &o);
    await(install, &o);
    DWORD exitCode = 1;
    GetExitCodeProcess(o.process, &exitCode);
    {
        WCHAR line[32];
        SIZE_T pos = 0;
        static const WCHAR sp[] = {' ', 0};
        wappend(line, &pos, msgExit);
        wappend(line, &pos, sp);
        wappendnum(line, &pos, exitCode);
        logw(install, line);
    }
    if (o.cancelled) {
        logf(install, msgCancel, 0, 0);
        result = ERROR_INSTALL_USEREXIT;
    } else {
        result = exitCode == 0 ? ERROR_SUCCESS : ERROR_INSTALL_FAILURE;
    }
done:
    if (o.process) {
        CloseHandle(o.process);
    }
    if (o.job) {
        // Closing the last handle of the job ends what is left in it.
        CloseHandle(o.job);
    }
    if (writeEnd) {
        CloseHandle(writeEnd);
    }
    if (o.pipe) {
        CloseHandle(o.pipe);
    }
    if (o.event) {
        CloseHandle(o.event);
    }
    if (attributes) {
        DeleteProcThreadAttributeList(attributes);
        HeapFree(heap, 0, attributes);
    }
    HeapFree(heap, 0, o.buf);
    HeapFree(heap, 0, cmd);
    HeapFree(heap, 0, data);
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
//     a|<account>|<account template>
//     ...
//
// op is `+` (grant), `-` (rollback), `c` (commit) or `x` (the product is
// removed). kind is `f` (file or folder) or `s` (service), mask and flags are hexadecimal access rights and
// ACE flags, the account is a SID string, an SDDL alias such as NS, a service
// manager name such as LocalSystem, or a name for LookupAccountNameW
// (`.\name` is a local account), and the object is a path or a service name.
//
// A grant adds the rights to the account's explicit allow ACE with the same
// inheritance flags, or adds such an ACE after the last explicit ACE, and
// keeps every other ACE, the inherited ones included. Before it changes an
// object it records the rights it adds, which the account did not have, in
// HKCU\Software\msi-builder\GrantRollback\<ProductCode> under the entry's
// number (HKCU of the account the action runs as: LocalSystem's hive in a
// per-machine install). The rollback takes
// exactly those rights away again and the commit deletes the record, so rights
// that were there before the install survive a rollback.
//
// An account that comes from a property, such as [SVC_ACCOUNT], is
// remembered: the commit action stores the account of an `a` entry under its
// template in HKLM\SOFTWARE\msi-builder\GrantAccounts\<ProductCode>, and a
// maintenance install (a repair from Apps & Features, a self-repair, a
// feature added later, a patch), whose command line does not have the
// property, grants to the remembered account when every property in the
// template is empty. Only an account that was never remembered is skipped
// there. The commit action of the removal of the product (`x`) deletes the
// key. Log lines read `BuilderGrant: ...`.

static const WCHAR propGrants[] = u"BuilderGrants";
static const WCHAR propProductCode[] = u"ProductCode";
static const WCHAR propExec[] = u"BuilderGrantExec";
static const WCHAR propRollback[] = u"BuilderGrantRollback";
static const WCHAR propCommit[] = u"BuilderGrantCommit";
static const WCHAR rollbackKey[] = u"Software\\msi-builder\\GrantRollback\\";
static const WCHAR accountsKey[] = u"SOFTWARE\\msi-builder\\GrantAccounts\\";
static const WCHAR condRemoveAll[] = u"REMOVE~=\"ALL\"";
static const WCHAR msgRemembered[] = u"the account is empty in a maintenance install; granting to the remembered account";
static const WCHAR msgRemember[] = u"cannot remember the account of";
static const WCHAR msgGrantData[] = u"malformed grant data";
static const WCHAR msgNoAccount[] = u"no account to grant rights to";
static const WCHAR msgSkipEmpty[] = u"the account is empty in a maintenance install and none is remembered; skipped";
static const WCHAR propInstalled[] = u"Installed";
static const WCHAR msgAccountType[] = u"the account names a domain, a deleted account or nothing";
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

// HKEY_CURRENT_USER and HKEY_LOCAL_MACHINE, which winreg.h defines as the
// sign-extended 0x80000001 and 0x80000002.
#define HKCU_ ((HANDLE)(INTPTR)(int)0x80000001)
#define HKLM_ ((HANDLE)(INTPTR)(int)0x80000002)

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

// maintenance reports whether the product is installed already: a repair, a
// reinstall, a patch or a feature change.
static int maintenance(MSIHANDLE install)
{
    WCHAR *installed = property(install, propInstalled);
    HeapFree(GetProcessHeap(), 0, installed);
    return installed != 0;
}

// accountsPath returns HKLM's key of the remembered accounts of a product.
static int accountsPath(wbuf *path, const WCHAR *product)
{
    return wbufAdd(path, accountsKey, wlen(accountsKey)) && wbufAdd(path, product, wlen(product));
}

// remembered returns the account an earlier install remembered for an
// account template, or 0.
static WCHAR *remembered(const WCHAR *product, const WCHAR *template)
{
    HANDLE heap = GetProcessHeap(), key = 0;
    wbuf path = {0, 0, 0};
    WCHAR *value = 0;
    if (accountsPath(&path, product) && !RegOpenKeyExW(HKLM_, path.s, 0, KEY_QUERY_VALUE, &key)) {
        DWORD kind = 0, size = 0;
        if (!RegQueryValueExW(key, template, 0, &kind, 0, &size) && kind == REG_SZ && size) {
            // The zeroed extra character ends a value stored without one.
            value = HeapAlloc(heap, HEAP_ZERO_MEMORY, (SIZE_T)size + 2 * sizeof(WCHAR));
            if (value && (RegQueryValueExW(key, template, 0, &kind, (BYTE *)value, &size) || kind != REG_SZ)) {
                HeapFree(heap, 0, value);
                value = 0;
            }
        }
        RegCloseKey(key);
    }
    HeapFree(heap, 0, path.s);
    return value;
}

// literal returns the template without its [...] references, which is what
// it formats to when every property in it is empty.
static WCHAR *literal(const WCHAR *template)
{
    WCHAR *out = HeapAlloc(GetProcessHeap(), 0, (wlen(template) + 1) * sizeof(WCHAR));
    if (!out) {
        return 0;
    }
    SIZE_T n = 0;
    int depth = 0;
    for (; *template; template++) {
        if (*template == '[') {
            depth++;
        } else if (*template == ']' && depth) {
            depth--;
        } else if (!depth) {
            out[n++] = *template;
        }
    }
    out[n] = 0;
    return out;
}

static int hasBracket(const WCHAR *s)
{
    for (; *s; s++) {
        if (*s == '[') {
            return 1;
        }
    }
    return 0;
}

// scheduleEntry formats one line of BuilderGrants and appends it to out. It
// returns 0 when the install has to fail.
static int scheduleEntry(MSIHANDLE install, const WCHAR *product, WCHAR *line, wbuf *out)
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
    WCHAR *acct = format(install, account), *obj = format(install, object), *kept = 0;
    int ok = 0;
    if (!acct || !obj) {
        logft(install, tmplGrant, msgFormat, acct ? object : account, 0);
    } else {
        WCHAR *name = trim(acct), *lit = hasBracket(account) ? literal(account) : 0;
        int again = maintenance(install);
        // `.\[SVC_USER]` with an empty property is as unset as `[SVC_USER]`.
        if (*name && lit && wieq(name, trim(lit))) {
            *name = 0;
        }
        HeapFree(GetProcessHeap(), 0, lit);
        if (!*name && again && (kept = remembered(product, account)) != 0 && *trim(kept)) {
            // A property set on the command line of the first install is
            // not there in a repair from Apps & Features, a self-repair or
            // a patch, which replace files and add components.
            name = trim(kept);
            logft(install, tmplGrant, msgRemembered, obj, 0);
        }
        if (!*name && again) {
            logft(install, tmplGrant, msgSkipEmpty, obj, 0);
            ok = 1;
        } else if (!*name) {
            logft(install, tmplGrant, msgNoAccount, obj, 0);
        } else if (hasControl(name, 1)) {
            logft(install, tmplGrant, msgBadAccount, name, 0);
        } else if (hasControl(obj, 0) || !*obj) {
            logft(install, tmplGrant, msgBadObject, obj, 0);
        } else {
            static const WCHAR remember[] = u"\na|";
            ok = wbufAdd(out, nl, 1) && wbufAdd(out, spec, wlen(spec)) && wbufAdd(out, bar, 1) && wbufAdd(out, name, wlen(name)) &&
                 wbufAdd(out, bar, 1) && wbufAdd(out, obj, wlen(obj));
            if (ok && hasBracket(account) && !hasControl(account, 1)) {
                ok = wbufAdd(out, remember, 3) && wbufAdd(out, name, wlen(name)) && wbufAdd(out, bar, 1) && wbufAdd(out, account, wlen(account));
            }
            if (!ok) {
                logft(install, tmplGrant, msgNoMemory, 0, 0);
            }
        }
    }
    HeapFree(GetProcessHeap(), 0, acct);
    HeapFree(GetProcessHeap(), 0, obj);
    HeapFree(GetProcessHeap(), 0, kept);
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
    if (MsiEvaluateConditionW(install, condRemoveAll) == MSICONDITION_TRUE) {
        // The removal only forgets the remembered accounts, in the commit
        // action, so a failed removal keeps them.
        out.s[0] = 'x';
        result = MsiSetPropertyW(install, propCommit, out.s) == ERROR_SUCCESS ? ERROR_SUCCESS : ERROR_INSTALL_FAILURE;
        goto done;
    }
    for (WCHAR *line = list; line && *line;) {
        WCHAR *next = field(line, '\n');
        if (!scheduleEntry(install, product, line, &out)) {
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
    } else if (use == SidTypeDomain || use == SidTypeDeletedAccount || use == SidTypeInvalid || use == SidTypeUnknown) {
        logft(install, tmplGrant, msgAccountType, account, 0);
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
// grant, the rights it is about to add are recorded under valueName in key
// (keyErr is why key is 0) before the object changes. It returns 0 on an
// error.
static int applyEntry(MSIHANDLE install, WCHAR *entry, int grant, HANDLE key, long keyErr, const WCHAR *valueName)
{
    HANDLE heap = GetProcessHeap();
    DWORD mask = 0, flags = 0;
    int type = entry[0] == 's' ? SE_SERVICE : SE_FILE_OBJECT;
    WCHAR *p = (entry[0] == 'f' || entry[0] == 's') && entry[1] == '|' ? hexfield(entry + 2, &mask) : 0;
    p = p ? hexfield(p, &flags) : 0;
    WCHAR *account = p, *object = p ? field(p, '|') : 0;
    if (!object || !*object || !*account || !mask || (flags & ~AUTHORED_INHERIT_FLAGS)) {
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
    if (grant) {
        long regErr = key ? RegSetValueExW(key, valueName, 0, REG_DWORD, (const BYTE *)&changed, sizeof changed) : keyErr;
        if (regErr) {
            logft(install, tmplGrant, msgRecord, object, (DWORD)regErr);
            goto done;
        }
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

// rememberEntry stores the account of an `a` entry under its template. The
// commit action does it, so a failed install or repair leaves the account
// that was remembered before. It never fails: without it, only a
// maintenance install without the property skips the grant.
static void rememberEntry(MSIHANDLE install, WCHAR *entry, const WCHAR *product)
{
    HANDLE heap = GetProcessHeap(), key = 0;
    WCHAR *account = entry + 2, *template = field(account, '|');
    wbuf path = {0, 0, 0};
    if (!template || !*template || !*account) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        return;
    }
    if (!accountsPath(&path, product)) {
        logft(install, tmplGrant, msgNoMemory, 0, 0);
        return;
    }
    long err = RegCreateKeyExW(HKLM_, path.s, 0, 0, 0, KEY_SET_VALUE, 0, &key, 0);
    if (!err) {
        err = RegSetValueExW(key, template, 0, REG_SZ, (const BYTE *)account, (DWORD)((wlen(account) + 1) * sizeof(WCHAR)));
        RegCloseKey(key);
    }
    if (err) {
        logft(install, tmplGrant, msgRemember, template, (DWORD)err);
    }
    HeapFree(heap, 0, path.s);
}

__declspec(dllexport) UINT WINAPI BuilderGrant(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
    WCHAR *data = property(install, propData);
    if (!data || (data[0] != '+' && data[0] != '-' && data[0] != 'c' && data[0] != 'x')) {
        logft(install, tmplGrant, msgGrantData, 0, 0);
        HeapFree(heap, 0, data);
        return data ? ERROR_INSTALL_FAILURE : ERROR_SUCCESS;
    }
    WCHAR op = data[0];
    WCHAR *entries = field(data + 1, '\n');
    wbuf path = {0, 0, 0};
    if (op == 'x') {
        if (accountsPath(&path, data + 1)) {
            RegDeleteTreeW(HKLM_, path.s);
        }
        HeapFree(heap, 0, path.s);
        HeapFree(heap, 0, data);
        return ERROR_SUCCESS;
    }
    if (!wbufAdd(&path, rollbackKey, wlen(rollbackKey)) || !wbufAdd(&path, data + 1, wlen(data + 1))) {
        HeapFree(heap, 0, data);
        return op == '+' ? ERROR_INSTALL_FAILURE : ERROR_SUCCESS;
    }
    HANDLE key = 0;
    long keyErr = 0;
    UINT result = ERROR_SUCCESS;
    if (op == '+') {
        // A record left by an install that never reached its commit (or
        // ran with rollback disabled, which skips commit actions too) is
        // stale.
        RegDeleteTreeW(HKCU_, path.s);
        if (entries && (keyErr = RegCreateKeyExW(HKCU_, path.s, 0, 0, 0, KEY_QUERY_VALUE | KEY_SET_VALUE, 0, &key, 0)) != 0) {
            key = 0;
        }
    } else if (op == '-' && RegOpenKeyExW(HKCU_, path.s, 0, KEY_QUERY_VALUE, &key)) {
        key = 0;
    }
    DWORD index = 0;
    for (WCHAR *entry = entries; entry && *entry; index++) {
        WCHAR *next = field(entry, '\n');
        WCHAR name[12];
        SIZE_T pos = 0;
        int remember = entry[0] == 'a' && entry[1] == '|';
        wappendnum(name, &pos, index);
        // The commit remembers the accounts; a rollback undoes every grant
        // it can and never fails.
        if (op == 'c') {
            if (remember) {
                rememberEntry(install, entry, data + 1);
            }
        } else if (!remember && !applyEntry(install, entry, op == '+', key, keyErr, name) && op == '+') {
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

// Closing running copies of package files (spec 17.8). BuilderCloseSchedule,
// an immediate action before InstallValidate, reads the property
// BuilderCloseApps that the builder writes, one entry per line:
//
//     <condition> US <timeout>| US <path>
//
// (US is U+001F; timeout is milliseconds in hexadecimal). For every entry
// whose condition is true it formats the path, finds the processes whose
// image is that file, posts WM_CLOSE to their windows and waits up to the
// timeout for them to exit. It runs as the installing user, so it can reach
// only the windows on the desktop of its custom action server and only
// processes the user may open. The deferred action BuilderCloseExec (entry
// point BuilderClose) runs as LocalSystem after StopServices and before
// RemoveFiles and ends every process still running from one of the paths in
// its CustomActionData, `|`-separated, in any session. That list is built by
// type-51 actions of the engine, a reset to `|` and then one per file that
// terminates, not by the schedule action: the impersonated custom action server runs with the
// user's token, so what it hands to LocalSystem must not decide which
// processes are ended. A process matches by its full image path, compared
// without regard to case, so a copy of the program in another folder is
// left alone. Neither action fails the install: a file that is still in use
// goes through the engine's files-in-use handling as before. Log lines read
// `BuilderClose: ...`.

static const WCHAR tmplClose[] = u"BuilderClose: [1]";
static const WCHAR propCloseApps[] = u"BuilderCloseApps";
static const WCHAR msgCloseData[] = u"malformed close data";
static const WCHAR msgSnapshot[] = u"cannot list the running processes";
static const WCHAR msgAsked[] = u"asked to close";
static const WCHAR msgNoWindow[] = u"has no window to close on this desktop";
static const WCHAR msgExited[] = u"exited";
static const WCHAR msgStillRunning[] = u"still running after the timeout";
static const WCHAR msgEnded[] = u"ended";
static const WCHAR msgGone[] = u"exited before it was ended";
static const WCHAR msgNotGone[] = u"was ended but has not exited yet";
static const WCHAR msgEndFailed[] = u"cannot end";
static const WCHAR msgCannotOpen[] = u"cannot open the process of";
static const WCHAR msgNoneRunning[] = u"not running";
static const WCHAR msgClosePid[] = u"process ";

// PROCESSENTRY32W as tlhelp32.h lays it out: 556 bytes on x86, 568 on x64
// and arm64.
typedef struct {
    DWORD dwSize;
    DWORD cntUsage;
    DWORD th32ProcessID;
    SIZE_T th32DefaultHeapID;
    DWORD th32ModuleID;
    DWORD cntThreads;
    DWORD th32ParentProcessID;
    long pcPriClassBase;
    DWORD dwFlags;
    WCHAR szExeFile[260];
} PROCESSENTRY32W;

_Static_assert(sizeof(PROCESSENTRY32W) == (sizeof(void *) == 4 ? 556 : 568), "PROCESSENTRY32W layout");

typedef void *HWND;
typedef BOOL(WINAPI *WNDENUMPROC)(HWND, INTPTR);

// kernel32.dll
HANDLE WINAPI CreateToolhelp32Snapshot(DWORD, DWORD);
BOOL WINAPI Process32FirstW(HANDLE, PROCESSENTRY32W *);
BOOL WINAPI Process32NextW(HANDLE, PROCESSENTRY32W *);
HANDLE WINAPI OpenProcess(DWORD, BOOL, DWORD);
BOOL WINAPI QueryFullProcessImageNameW(HANDLE, DWORD, WCHAR *, DWORD *);
int WINAPI CompareStringOrdinal(const WCHAR *, int, const WCHAR *, int, BOOL);

// user32.dll
BOOL WINAPI EnumWindows(WNDENUMPROC, INTPTR);
DWORD WINAPI GetWindowThreadProcessId(HWND, DWORD *);
BOOL WINAPI PostMessageW(HWND, UINT, INTPTR, INTPTR);
int WINAPI GetClassNameW(HWND, WCHAR *, int);
BOOL WINAPI IsWindowVisible(HWND);
HWND WINAPI GetWindow(HWND, UINT);

enum {
    TH32CS_SNAPPROCESS = 2,
    PROCESS_TERMINATE = 0x1,
    PROCESS_QUERY_LIMITED_INFORMATION = 0x1000,
    SYNCHRONIZE = 0x100000,
    CSTR_EQUAL = 2,
    WM_CLOSE = 0x10,
    GW_OWNER = 4,
    // The longest path QueryFullProcessImageNameW returns, in characters.
    IMAGE_PATH = 32768,
    // How long the deferred action waits for an ended process to go away.
    TERMINATE_WAIT_MS = 10000,
    // Processes of one file handled in one pass. The schedule action asks
    // the first CLOSE_MAX copies only; the deferred action takes further
    // passes, up to CLOSE_PASSES, until it finds fewer.
    CLOSE_MAX = 64,
    CLOSE_PASSES = 16,
};

// closeTarget is one running copy of a file.
typedef struct {
    HANDLE process;
    DWORD pid;
    DWORD windows; // windows asked to close
    DWORD visible; // of them, visible ones without an owner
} closeTarget;

static int wieqi(const WCHAR *a, const WCHAR *b)
{
    return CompareStringOrdinal(a, -1, b, -1, 1) == CSTR_EQUAL;
}

// baseName returns the text after the last backslash.
static const WCHAR *baseName(const WCHAR *path)
{
    const WCHAR *name = path;
    for (; *path; path++) {
        if (*path == '\\') {
            name = path + 1;
        }
    }
    return name;
}

// findRunning opens every process whose image is path with the given access
// and returns how many it put into targets. image is a scratch buffer of
// IMAGE_PATH characters.
static DWORD findRunning(MSIHANDLE install, const WCHAR *path, DWORD access, closeTarget *targets, WCHAR *image)
{
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) {
        logft(install, tmplClose, msgSnapshot, 0, GetLastError());
        return 0;
    }
    const WCHAR *name = baseName(path);
    DWORD n = 0;
    PROCESSENTRY32W entry;
    memset(&entry, 0, sizeof entry);
    entry.dwSize = sizeof entry;
    for (BOOL more = Process32FirstW(snap, &entry); more && n < CLOSE_MAX; more = Process32NextW(snap, &entry)) {
        entry.szExeFile[259] = 0;
        if (!wieqi(entry.szExeFile, name)) {
            continue;
        }
        HANDLE process = OpenProcess(access | PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE, 0, entry.th32ProcessID);
        if (!process) {
            // Another user's program, from the immediate action, or one
            // that exited in between: the deferred action ends the first.
            logft(install, tmplClose, msgCannotOpen, entry.szExeFile, GetLastError());
            continue;
        }
        DWORD size = IMAGE_PATH;
        if (QueryFullProcessImageNameW(process, 0, image, &size) && wieqi(image, path)) {
            targets[n].process = process;
            targets[n].pid = entry.th32ProcessID;
            targets[n].windows = 0;
            targets[n].visible = 0;
            n++;
        } else {
            CloseHandle(process);
        }
    }
    CloseHandle(snap);
    return n;
}

static void logPid(MSIHANDLE install, const WCHAR *what, DWORD pid, const WCHAR *path, DWORD code)
{
    static const WCHAR open[] = u" (", close[] = u")";
    WCHAR *s = HeapAlloc(GetProcessHeap(), 0, (wlen(path) + 32) * sizeof(WCHAR));
    if (s) {
        SIZE_T pos = 0;
        wappend(s, &pos, msgClosePid);
        wappendnum(s, &pos, pid);
        wappend(s, &pos, open);
        wappend(s, &pos, path);
        wappend(s, &pos, close);
        logft(install, tmplClose, s, what, code);
        HeapFree(GetProcessHeap(), 0, s);
    }
}

typedef struct {
    closeTarget *targets;
    DWORD n;
    // hidden is 0 in the first pass, which posts to visible windows that no
    // other window owns, and 1 in the second, which posts to the hidden
    // windows of targets that have no such window, as a program that only
    // sits in the notification area has.
    int hidden;
} closeWindows;

// postClose is the EnumWindows callback: it posts WM_CLOSE to a top-level
// window of a target process. The input method windows that the system
// gives every GUI thread are left alone, and so is a console window, which
// the console host may report as the program's: closing it ends every
// process attached to the console, not only this one.
static BOOL WINAPI postClose(HWND window, INTPTR param)
{
    static const WCHAR ime[] = u"IME", msctf[] = u"MSCTFIME UI", console[] = u"ConsoleWindowClass";
    closeWindows *w = (closeWindows *)param;
    DWORD pid = 0;
    GetWindowThreadProcessId(window, &pid);
    for (DWORD i = 0; i < w->n; i++) {
        if (w->targets[i].pid != pid) {
            continue;
        }
        if (w->hidden ? w->targets[i].visible != 0 || IsWindowVisible(window) : !IsWindowVisible(window) || GetWindow(window, GW_OWNER)) {
            break;
        }
        WCHAR cls[24];
        int len = GetClassNameW(window, cls, 24);
        if (len > 0 && (wieqi(cls, ime) || wieqi(cls, msctf) || wieqi(cls, console))) {
            break;
        }
        if (PostMessageW(window, WM_CLOSE, 0, 0)) {
            w->targets[i].windows++;
            w->targets[i].visible += !w->hidden;
        }
        break;
    }
    return 1;
}

// closeEntry asks the running copies of path to close and waits for them.
// It returns 0 after a cancel.
static int closeEntry(MSIHANDLE install, const WCHAR *path, DWORD timeout, closeTarget *targets, WCHAR *image)
{
    DWORD n = findRunning(install, path, 0, targets, image);
    if (!n) {
        logft(install, tmplClose, path, msgNoneRunning, 0);
        return 1;
    }
    closeWindows w = {targets, n, 0};
    EnumWindows(postClose, (INTPTR)&w);
    w.hidden = 1;
    EnumWindows(postClose, (INTPTR)&w);
    unsigned long long deadline = GetTickCount64() + timeout;
    int ok = 1;
    for (DWORD i = 0; i < n && ok; i++) {
        if (!targets[i].windows) {
            logPid(install, msgNoWindow, targets[i].pid, path, 0);
            continue;
        }
        logPid(install, msgAsked, targets[i].pid, path, 0);
        DWORD wait = WAIT_TIMEOUT;
        for (;;) {
            unsigned long long now = GetTickCount64();
            DWORD slice = now >= deadline ? 0 : deadline - now > CANCEL_POLL_MS ? CANCEL_POLL_MS : (DWORD)(deadline - now);
            wait = WaitForSingleObject(targets[i].process, slice);
            if (wait != WAIT_TIMEOUT || slice == 0) {
                break;
            }
            if (cancelled(install)) {
                ok = 0;
                break;
            }
        }
        if (ok) {
            logPid(install, wait == WAIT_OBJECT_0 ? msgExited : msgStillRunning, targets[i].pid, path, 0);
        }
    }
    for (DWORD i = 0; i < n; i++) {
        CloseHandle(targets[i].process);
    }
    return ok;
}

// closeLine handles one line of BuilderCloseApps. It returns -1 for
// malformed data, 0 after a cancel and 1 otherwise.
static int closeLine(MSIHANDLE install, WCHAR *line, closeTarget *targets, WCHAR *image)
{
    WCHAR *spec = field(line, 0x1F);
    WCHAR *tmpl = spec ? field(spec, 0x1F) : 0;
    DWORD timeout = 0;
    if (!tmpl || !hexfield(spec, &timeout)) {
        logft(install, tmplClose, msgCloseData, 0, 0);
        return -1;
    }
    if (*line) {
        int cond = MsiEvaluateConditionW(install, line);
        if (cond == 0) {
            return 1;
        }
        if (cond != MSICONDITION_TRUE && cond != MSICONDITION_NONE) {
            logft(install, tmplClose, msgCondition, line, 0);
            return 1;
        }
    }
    WCHAR *path = format(install, tmpl);
    int ok = 1;
    if (!path || !*path || hasControl(path, 1)) {
        logft(install, tmplClose, msgFormat, tmpl, 0);
    } else {
        ok = closeEntry(install, path, timeout, targets, image);
    }
    HeapFree(GetProcessHeap(), 0, path);
    return ok;
}

__declspec(dllexport) UINT WINAPI BuilderCloseSchedule(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
    WCHAR *list = property(install, propCloseApps);
    WCHAR *image = HeapAlloc(heap, 0, IMAGE_PATH * sizeof(WCHAR));
    closeTarget *targets = HeapAlloc(heap, 0, CLOSE_MAX * sizeof(closeTarget));
    UINT result = ERROR_SUCCESS;
    if (!image || !targets) {
        logft(install, tmplClose, msgNoMemory, 0, 0);
    }
    for (WCHAR *line = list; image && targets && line && *line;) {
        WCHAR *next = field(line, '\n');
        int ok = closeLine(install, line, targets, image);
        if (ok < 0) {
            break;
        }
        if (ok == 0) {
            result = ERROR_INSTALL_USEREXIT;
            break;
        }
        line = next;
    }
    HeapFree(heap, 0, targets);
    HeapFree(heap, 0, image);
    HeapFree(heap, 0, list);
    return result;
}

// endRunning ends the processes of one path and returns how many it found.
static DWORD endRunning(MSIHANDLE install, const WCHAR *path, closeTarget *targets, WCHAR *image)
{
    DWORD n = findRunning(install, path, PROCESS_TERMINATE, targets, image);
    for (DWORD i = 0; i < n; i++) {
        HANDLE process = targets[i].process;
        if (WaitForSingleObject(process, 0) == WAIT_OBJECT_0) {
            logPid(install, msgGone, targets[i].pid, path, 0);
        } else if (!TerminateProcess(process, ERROR_INSTALL_USEREXIT)) {
            logPid(install, msgEndFailed, targets[i].pid, path, GetLastError());
        } else {
            DWORD wait = WaitForSingleObject(process, TERMINATE_WAIT_MS);
            logPid(install, wait == WAIT_OBJECT_0 ? msgEnded : msgNotGone, targets[i].pid, path, 0);
        }
        CloseHandle(process);
    }
    return n;
}

__declspec(dllexport) UINT WINAPI BuilderClose(MSIHANDLE install)
{
    HANDLE heap = GetProcessHeap();
    WCHAR *data = property(install, propData);
    WCHAR *image = HeapAlloc(heap, 0, IMAGE_PATH * sizeof(WCHAR));
    closeTarget *targets = HeapAlloc(heap, 0, CLOSE_MAX * sizeof(closeTarget));
    if (!image || !targets) {
        logft(install, tmplClose, msgNoMemory, 0, 0);
    }
    // The data starts with the builder's empty seed and ends with `|`.
    for (WCHAR *path = data; image && targets && path;) {
        WCHAR *next = field(path, '|');
        if (*path) {
            DWORD n = endRunning(install, path, targets, image);
            if (!n) {
                logft(install, tmplClose, path, msgNoneRunning, 0);
            }
            for (int pass = 1; n == CLOSE_MAX && pass < CLOSE_PASSES; pass++) {
                n = endRunning(install, path, targets, image);
            }
        }
        path = next;
    }
    HeapFree(heap, 0, targets);
    HeapFree(heap, 0, image);
    HeapFree(heap, 0, data);
    return ERROR_SUCCESS;
}

BOOL WINAPI _DllMainCRTStartup(HANDLE instance, DWORD reason, void *reserved)
{
    (void)instance;
    (void)reason;
    (void)reserved;
    return 1;
}
