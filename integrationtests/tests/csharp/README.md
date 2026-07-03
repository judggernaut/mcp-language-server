# C# Integration Tests

This directory contains integration tests for `csharp-ls`, the C# language server.

## Prerequisites

These tests require the .NET SDK and `csharp-ls` to be installed:

```bash
dotnet tool install --global csharp-ls
```

`csharp-ls` is installed as a dotnet tool, so make sure `~/.dotnet/tools` is on your `PATH`.

## csharp-ls version and interop quirks

These tests are run against **csharp-ls 0.25.0**. A couple of behaviors are specific to
csharp-ls and shape how the C# test suite differs from the other languages:

- **`workspace/symbol` only reports bare names for types.** For classes, interfaces and
  enums, `SymbolInformation.Name` is the plain identifier (e.g. `"SharedClass"`), which
  matches this fork's exact-name lookup used by the `definition` and `references` tools.
  For methods and fields, csharp-ls reports the full signature as the name instead (e.g.
  `"string SharedClass.Method()"` or `"string SharedConstants.SharedConstant"`). Since the
  fork's method-matching logic checks for a `.MethodName` suffix, the trailing `()` on
  csharp-ls's method names means bare-name lookups for methods and fields never match, even
  though `workspace_symbol`'s own fuzzy search finds them fine. Because of this, the
  `definition` and `references` test suites here only cover class/interface/enum symbols.
  Position-based tools (`hover`, `rename_symbol`) are unaffected, since they don't go
  through `workspace/symbol`, and their tests do cover methods, fields, constants and
  variables.
- **Type-level `definition` results span the whole file.** For classes, interfaces and
  enums, the `Location` returned by `workspace/symbol` covers the entire containing file
  rather than just the symbol's body. This means `read_definition` for a type returns the
  full file content rather than a tightly scoped snippet, unlike Go/Rust/Python/TypeScript.
  The definition snapshots in this suite reflect that whole-file output; it is expected,
  not a bug in this fork's code.

While these tests may pass with other versions of csharp-ls, compatibility is not
guaranteed, particularly around the exact diagnostic message text (`FileWithError`'s
snapshot is skipped for this reason, matching the same practice used for rust-analyzer).
