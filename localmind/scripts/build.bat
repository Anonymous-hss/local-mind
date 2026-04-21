@echo off
REM Build script for LocalMind (Windows)
REM Usage: scripts\build.bat [core|extension|all]

setlocal enabledelayedexpansion

set ROOT_DIR=%~dp0..
set TARGET=%1
if "%TARGET%"=="" set TARGET=all

if "%TARGET%"=="core" goto build_core
if "%TARGET%"=="extension" goto build_extension
if "%TARGET%"=="all" goto build_all
goto usage

:build_core
echo Building Core Engine (Go)...
cd /d "%ROOT_DIR%\packages\core"
go build -o bin\localmind.exe .\cmd\localmind
echo Core engine built: packages\core\bin\localmind.exe
goto :eof

:build_extension
echo Building VS Code Extension...
cd /d "%ROOT_DIR%\packages\extension"
call npm install
call npm run compile
echo Extension compiled: packages\extension\out\
goto :eof

:build_all
call :build_core
call :build_extension
echo Build complete!
goto :eof

:usage
echo Usage: %0 [core^|extension^|all]
exit /b 1
