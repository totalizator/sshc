# Android / Termux

The prebuilt `linux/*` release binaries do **not** run under
[Termux](https://termux.dev/). A cross-compiled `GOOS=linux` binary is the wrong
target for Android: its loader rejects such a binary in three successive stages.

1. **`e_type: 2`** — default Go static builds are `ET_EXEC`; Android's loader
   only accepts position-independent executables (`ET_DYN`). The release build
   now passes `-buildmode=pie`, which clears this stage — but it is *necessary,
   not sufficient*.
2. **TLS underalignment** — bionic on arm64 needs the TLS segment aligned to ≥64
   bytes; Go's `GOOS=linux` linker emits 8. `termux-elf-cleaner` can patch this
   in place, but it shouldn't be required.
3. **Doubled `argv`** — even after the above, the binary receives its own path as
   `argv[1]` under Android's loader, so the CLI treats it as an unknown
   subcommand (`unknown command "/data/.../sshc"`). This is not patchable from
   outside the binary.

## Build from source instead

In Termux, Go targets Android (`android/arm64`) natively, which avoids all three
problems:

```sh
pkg install golang git openssh
git clone https://github.com/totalizator/sshc.git
cd sshc && go build -o sshc .
./sshc --version
```

`openssh` provides the `ssh` binary that sshc execs when you connect.

## Prebuilt Android binary?

Shipping a prebuilt `GOOS=android` CI artifact would need the Android NDK and
`CGO_ENABLED=1` in the release workflow. Until demand justifies the NDK
toolchain, building from source is the pragmatic answer. The `-buildmode=pie`
change to the linux build stays regardless — it enables ASLR and matches the
modern-distro default; it just isn't sufficient for Android on its own.
