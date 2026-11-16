# Dynamic MTSDF native bridge (Windows x64)

This is the source-only ABI1 / generator revision4 bridge used by DynamicMSDF.
Use Windows x64, Visual Studio 2022 C++ tools/Windows SDK, CMake>=3.20 and C++20.
From the MyGo repository root, in a Developer PowerShell:

```powershell
cmake -S native/mtsdf -B out/mtsdf -G "Visual Studio 17 2022" -A x64
cmake --build out/mtsdf --config Release --parallel
Get-FileHash out/mtsdf/Release/cloudy_mtsdf.dll -Algorithm SHA256
```

MSVC runtime, MSDFGen and Skia PathOps are linked statically; the DLL uses /MT.
No prebuilt library, VCPKG, FreeType, standalone msdfgen CLI or network download
is needed. The native target is Windows x64 only; other Go platforms retain
the unsupported-platform DynamicMSDF stub and native text fallback.

Ship the rebuilt DLL beside the application EXE, together with MSDFGen's
MIT license (msdfgen/LICENSE.txt) and Skia's BSD license (skia-source/LICENSE),
plus the application's/MyGo's licenses. Preserve source notices. Do not commit
the DLL, EXE, .lib, object files or build directory. The calling application
must pass the release DLL's SHA to OpenDynamicMSDF; this source lock deliberately
contains no machine path or DLL artifact SHA. A rebuild may produce a different
binary SHA (toolchain/linker timestamp/debug-path differences); rebuild and bind
the EXE/DLL as a pair. ABI1 is distinct from generator revision4 and MSDFGen
local_revision3. Validate cm_abi()==1 and cm_revision()==4 when packaging.

Provenance is in dependency-lock.json and the two source manifests. MSDFGen
v1.13 core includes the two recorded edge-selector local patches. The full
230-file frozen Skia PathOps subset is intentional: resolve-shape-geometry.cpp
calls Simplify and the 1024 coordinate mapping is part of accepted R4 geometry.
Do not drop PathOps or change cleanup/fill/normalization/range/scale algorithms
as a packaging cleanup. No tests, FontLab, benchmark or diagnostic CLI are part
of this native source integration.
