@echo off
REM Preloží bateria-hook.dll – knižnicu, ktorá vloží čas do hodín v paneli.
REM Stačí spustiť tento súbor dvojklikom (alebo v príkazovom riadku).
REM
REM Skúsi nájsť g++ (MinGW/MSYS2) alebo cl.exe (Visual Studio Build Tools).

setlocal
cd /d "%~dp0"

where g++ >nul 2>nul
if %errorlevel%==0 (
    echo Prekladam cez g++ ^(MinGW^)...
    g++ -O2 -s -shared -static -municode -o bateria-hook.dll hook.cpp -lkernel32 -luser32
    goto done
)

where cl >nul 2>nul
if %errorlevel%==0 (
    echo Prekladam cez cl ^(MSVC^)...
    cl /nologo /LD /O2 /EHsc hook.cpp /Fe:bateria-hook.dll kernel32.lib user32.lib
    del /q hook.obj >nul 2>nul
    goto done
)

echo.
echo Nenasiel sa ziadny prekladac C++.
echo Nainstaluj si jednu z tychto moznosti a spusti build.bat znova:
echo   1^) MSYS2 ^(zadarmo^): https://www.msys2.org/  potom v MSYS2 termine:
echo        pacman -S mingw-w64-x86_64-gcc
echo        a pridaj C:\msys64\mingw64\bin do PATH
echo   2^) Visual Studio Build Tools ^(zadarmo^):
echo        https://visualstudio.microsoft.com/downloads/  -> "Build Tools"
echo        potom spusti tento build.bat z "x64 Native Tools Command Prompt"
exit /b 1

:done
if exist bateria-hook.dll (
    echo.
    echo Hotovo: bateria-hook.dll
    echo Skopiruj ju do rovnakeho priecinka, kde mas bateria.exe, a spusti bateria.exe.
) else (
    echo.
    echo Preklad zlyhal.
    exit /b 1
)
endlocal
