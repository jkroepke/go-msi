# LANG-03 / builder-languages

GitHub Actions `windows-latest` runner (Windows Server 2025 10.0.26100,
image windows-2025-vs2026 20261021.278.1, en-US), CI run
https://github.com/jkroepke/msi-builder/actions/runs/40897563160
(job 123530815048, commit a97faa5), step "go test (language transform
results)":

```text
=== RUN   TestLanguagesInstallLANG03
    languages_windows_test.go:54: LANG-03: ProductLanguage=1036 registered Language "1036"
    languages_windows_test.go:54: LANG-03: ProductLanguage=1031 registered Language "1031"
--- PASS: TestLanguagesInstallLANG03 (2.01s)
=== RUN   TestFullUILanguages
    languages_windows_test.go:95: LANG-03 UI:
        WINDOW: Weiter > | Abbrechen | < Zurück | DialogBmp | Der Setup-Assistent installiert MsiBuilderFullUI auf Ihrem Computer. Klicken Sie auf Weiter, um fortzufahren, oder auf Abbrechen, um den Setup-Assistenten zu beenden. | Willkommen beim Setup-Assistenten für MsiBuilderFullUI
        PRESSED: Weiter >
        WINDOW: Warnings | info | Debug | < Zurück | Weiter > | Abbrechen | BannerBmp | Log level | Log level
        PRESSED: Weiter >
        WINDOW: Durchsuchen... | Zurücksetzen | Speicherbedarf | < Zurück | Weiter > | Abbrechen | BannerBmp | Wählen Sie aus, wie die Features installiert werden sollen. | Klicken Sie auf die Symbole in der Struktur, um die Installationsart der Features zu ändern. | Dieses Feature benötigt 44KB auf Ihrer Festplatte. | C:\Program Files\MsiBuilderFullUI\ | Speicherort: | Benutzerdefiniertes Setup
        PRESSED: Weiter >
        WINDOW: Installieren | Abbrechen | < Zurück | BannerBmp | Klicken Sie auf Installieren, um die Installation zu starten. Klicken Sie auf Zurück, um Ihre Einstellungen zu prüfen oder zu ändern. Klicken Sie auf Abbrechen, um den Assistenten zu beenden. | Bereit zur Installation von MsiBuilderFullUI
        PRESSED: Installieren
        WINDOW: Fertig stellen | < Zurück | DialogBmp | Abbrechen | Klicken Sie auf Fertig stellen, um den Setup-Assistenten zu beenden. | Der Setup-Assistent für MsiBuilderFullUI ist abgeschlossen | Viel Spaß mit MsiBuilderFullUI.
        PRESSED: Fertig stellen
        DRIVER: done
--- PASS: TestFullUILanguages (9.15s)
PASS
```

The package's Template ends in `;1033,1031,1036` (the test asserts it).
Both installs exit 0 and register the language that `ProductLanguage`
chose. On the en-US runner, `ProductLanguage=1031` applies the German
transform before the first dialog: every wizard text is German, from the
built-in translation, and the finish page shows the `de` entry's own
`finishText`. The log-level page keeps its English texts because they come
from the test configuration's `ui.dialogs`, which no transform changes.
