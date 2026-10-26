# Go API

go-msi can build a package from Go without a YAML file. The YAML file and
the Go API share one model: `config.Config` is what `config.Load` returns,
and `builder.Build` validates it the same way, so every rule of the
[YAML schema](design/yaml-schema.md) also applies to a config that Go code
builds.

```go
cfg := &config.Config{
	Schema:  config.SchemaV1,
	Product: config.Product{Name: "My App", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
	Package: config.Package{Platform: "x64"},
	Upgrade: &config.Upgrade{},
	Directories: []*config.Directory{{
		ID:       "ProgramFiles64Folder",
		Children: []*config.Directory{{ID: "INSTALLDIR", Name: "My App", Configurable: true}},
	}},
	Features: []*config.Feature{{ID: "Main", Title: "My App"}},
}

// Add every file under dist/, one component per file, below INSTALLDIR.
if _, err := cfg.AddTree(os.DirFS("."), "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
	return err
}

// With Output set, the package is streamed to the file.
res, err := builder.Build(cfg, builder.Options{Output: "my-app.msi"})
if err != nil {
	return fmt.Errorf("%w\n%s", err, res.Diagnostics)
}
```

## Where files come from

A relative `source` path is read, in this order of precedence, from:

1. `File.FS`, the file system of that one file (`AddTree` sets it);
2. `Config.FS`, for every file, icon, binary and binary cell of a custom
   table, for example an `embed.FS` or an `fstest.MapFS`;
3. `Config.BaseDir`, the directory of the YAML file, which `config.Load` sets.

In a relative path a backslash counts as a separator on every OS, and the
path is cleaned, so `bin\app.exe`, `./bin/app.exe` and `bin/app.exe` name
the same file. An absolute path is always read from the
operating system's file system. Payload files are streamed, never held in
memory, so the files of an `fs.FS` must implement `io.ReaderAt`, as those of
`os.DirFS`, `embed.FS` and `fstest.MapFS` do.

## Adding a folder tree

`Config.AddTree(fsys, root, dirID, opts)` adds every file under the folder
`root` below the existing directory `dirID`. A config that relies on the
shorthands, such as `installDir` or the default feature, gets those
directories and features from `config.ApplyDefaults`; call it before
`AddTree`. Use `"."` as `root` for the
whole file system; an empty `root` is an error.

- Each subdirectory that holds a file, directly or further down, becomes a
  child `Directory`. A subdirectory that already exists below `dirID` under
  the same long name, for example from an earlier `AddTree` or one you
  defined, is reused. Folders without any file are skipped.
- Each file becomes its own `Component` with that one file, so it has its
  own key path and an automatic GUID.
- The components join the feature `opts.Feature`.
- IDs are derived from `opts.Prefix` (default `tree`), `dirID`, `root` and
  the path below `root`. The same calls in the same order therefore give the
  same IDs, GUIDs and ProductCode on every build; with a fixed
  `Options.BuildTime` or `SOURCE_DATE_EPOCH` the package is byte-identical. Trees with a different
  `dirID`, `root` or `Prefix` never share a component or file ID; they share
  only the IDs of reused folders. Two trees that install a file to the same
  path, or a file where a folder is, are an error.
- A symlink or other special file below `root` is an error, and so is
  a name below `root` that Windows does not allow, such as one with `|` or
  `:`, a trailing dot or a device name like `nul.txt`. The folders of `root`
  itself are not installed; `root` only has to be a valid `fs.FS` path
  without `\` or `:`. On an error the config is unchanged.
- Harvested files have no short names, so `AddTree` refuses a package with
  `longFileNames: false`.

`AddTree` returns the new components, for example to set `Permissions` or
`AllowUnversionedExecutable` on some of them. The usual rules still apply to
each file: an executable without a version resource is an error unless it
is marked as allowed (RUN-02).

YAML files cannot harvest a folder; that is not in schema v1.
