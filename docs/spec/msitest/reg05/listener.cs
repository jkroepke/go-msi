// REG-05 listener: a hidden window registered for SHCNE_ASSOCCHANGED with
// SHChangeNotifyRegister. It appends one line per notification to the log
// and quits when the stop file appears.
using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows.Forms;

public class AssocListener : Form
{
    [StructLayout(LayoutKind.Sequential)]
    struct Entry { public IntPtr pidl; public bool recursive; }

    [DllImport("shell32.dll")]
    static extern uint SHChangeNotifyRegister(IntPtr hwnd, int sources, int events, uint msg, int count, ref Entry entry);
    [DllImport("shell32.dll")]
    static extern bool SHChangeNotifyDeregister(uint id);
    [DllImport("shell32.dll")]
    static extern int SHGetSpecialFolderLocation(IntPtr hwnd, int folder, out IntPtr pidl);

    const uint WmNotify = 0x0400 + 0x52;
    const int AssocChanged = 0x08000000;
    const int InterruptLevel = 0x1, ShellLevel = 0x2;

    readonly string log, stop;
    uint id;

    public AssocListener(string log, string stop)
    {
        this.log = log;
        this.stop = stop;
        var timer = new Timer { Interval = 200 };
        timer.Tick += (s, e) => { if (File.Exists(this.stop)) Close(); };
        timer.Start();
    }

    void Write(string line)
    {
        for (int i = 0; i < 20; i++)
        {
            try { File.AppendAllText(log, DateTime.Now.ToString("HH:mm:ss.fff") + " " + line + "\r\n"); return; }
            catch (IOException) { System.Threading.Thread.Sleep(50); }
        }
    }

    protected override void SetVisibleCore(bool value)
    {
        if (!IsHandleCreated) CreateHandle();
        base.SetVisibleCore(false);
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        IntPtr desktop;
        SHGetSpecialFolderLocation(IntPtr.Zero, 0, out desktop);
        var entry = new Entry { pidl = desktop, recursive = true };
        id = SHChangeNotifyRegister(Handle, InterruptLevel | ShellLevel, AssocChanged, WmNotify, 1, ref entry);
        Write("listener registered id=" + id + " session=" + Process.GetCurrentProcess().SessionId +
              " explorer=" + (Process.GetProcessesByName("explorer").Length > 0));
    }

    protected override void WndProc(ref Message m)
    {
        if (m.Msg == WmNotify)
            Write("EVENT 0x" + ((long)m.LParam & 0xffffffffL).ToString("x8"));
        base.WndProc(ref m);
    }

    protected override void OnFormClosed(FormClosedEventArgs e)
    {
        if (id != 0) SHChangeNotifyDeregister(id);
        Write("listener stopped");
        base.OnFormClosed(e);
    }

    public static void Run(string log, string stop)
    {
        Application.Run(new AssocListener(log, stop));
    }
}
