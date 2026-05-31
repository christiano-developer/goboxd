# docs/ai/issues.md
# Issues Log (Stage 1)

## 2026-05-31 · nsjail Python 3 dynamic library loading failure

**What we were trying to do:**
Execute sandboxed Python scripts inside `nsjail` by running the interpreter `/usr/bin/python3` inside the container runtime.

**What went wrong:**
The sandbox execution failed immediately with the error:
`python3: error while loading shared libraries: libpython3.11.so.1.0: cannot open shared object file: No such file or directory`

**How we resolved it:**
We discovered that `nsjail`'s custom isolation did not map the host system's library folder paths correctly into the running workspace. We resolved it by mounting the host root read-only using `--chroot /` and explicitly setting up read-only bindings for `/lib`, `/usr/lib`, and `/lib64`.

**What we learned:**
Standard runtimes/interpreters have dynamic linkages that require full system access paths, meaning custom minimal root jail directory mappings will fail unless system directories are bound read-only.

---

## 2026-05-31 · GCC compilation failing with missing stdio.h header

**What we were trying to do:**
Compile user-submitted C programs under the sandbox using `/usr/bin/gcc`.

**What went wrong:**
The compilation failed during execution with:
`fatal error: stdio.h: No such file or directory`

**How we resolved it:**
We checked the Dockerfile runtime dependencies and found that we only installed `gcc`. In Debian, the standard library header files are not bundled with the compiler but reside in `libc6-dev`. We added `libc6-dev` to the runtime stage package manager list in the `Dockerfile`.

**What we learned:**
Always install developer header packages alongside standard compilation toolchains in base container layers to ensure compiler environments are fully self-sufficient.
