@echo off
setlocal enabledelayedexpansion

if "%~1"=="" (
    set "ARG=local"
) else (
    set "ARG=%~1"
)

set SCRIPT_DIR=%~dp0
set LOCAL_BUILD=0

if /i "%ARG%"=="local" (
    set LOCAL_BUILD=1
    set VERSION=dev
    set RELEASE_DIR=%SCRIPT_DIR%bin
) else (
    set VERSION=%ARG%
    :: Auto-increment build number
    set BUILD_NUM=1
    if exist "%SCRIPT_DIR%releases" (
        for /d %%d in ("%SCRIPT_DIR%releases\!VERSION!-*") do (
            for /f "tokens=* delims=" %%n in ("%%~nxd") do (
                for /f "tokens=2 delims=-" %%b in ("%%n") do (
                    set /a CANDIDATE=%%b+1
                    if !CANDIDATE! gtr !BUILD_NUM! set BUILD_NUM=!CANDIDATE!
                )
            )
        )
    )
    set RELEASE_TAG=!VERSION!-!BUILD_NUM!
    set RELEASE_DIR=%SCRIPT_DIR%releases\!RELEASE_TAG!
)

if "%LOCAL_BUILD%"=="1" (
    echo.
    echo ========================================
    echo  nanotown local build
    echo ========================================
    echo.
) else (
    echo.
    echo ========================================
    echo  nanotown release build v!RELEASE_TAG!
    echo ========================================
    echo.
)

:: Check Go is installed
where go >nul 2>&1
if errorlevel 1 (
    echo ERROR: Go is not installed or not in PATH.
    exit /b 1
)

:: Build binary
echo [1/2] Compiling binary...
if not exist "%RELEASE_DIR%" mkdir "%RELEASE_DIR%"
go build -o "%RELEASE_DIR%\nt.exe" ./src
if errorlevel 1 (
    echo ERROR: Build failed.
    exit /b 1
)

:: Copy release files (release builds only)
if "%LOCAL_BUILD%"=="0" (
    echo [2/2] Assembling release...
    copy "%SCRIPT_DIR%LICENSE.md" "%RELEASE_DIR%\" >nul
    copy "%SCRIPT_DIR%README.md" "%RELEASE_DIR%\" >nul
) else (
    echo [2/2] Done.
)

echo.
echo ========================================
echo  Build complete!
echo ========================================
echo.
if "%LOCAL_BUILD%"=="1" (
    echo  Binary: %RELEASE_DIR%\nt.exe
) else (
    echo  Version:  %RELEASE_TAG%
    echo  Binary:   %RELEASE_DIR%\nt.exe
    echo  Release:  %RELEASE_DIR%\
)
echo.

endlocal
