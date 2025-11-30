@echo off
setlocal enabledelayedexpansion

cd /d "%~dp0"

echo.
echo === Tidying Go modules ===
go mod tidy

:: Устанавливаем окружение для сборки
echo.
echo === Setting PATH and environment ===
set "PATH=C:\Program Files\Go\bin;%PATH%"
set "PATH=C:\msys64\mingw64\bin;%PATH%"
set "PATH=%USERPROFILE%\go\bin;%PATH%"
set GOROOT=C:\Program Files\Go

set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
set CGO_CFLAGS=-IC:/msys64/mingw64/include
set CGO_LDFLAGS=-LC:/msys64/mingw64/lib

:: Пересобираем стандартную библиотеку (разово)
:: Если проблемы с toolchain — временно раскомментировать:
:: go install std

:: === ADDED: Embed icon into the executable ===
echo.
echo === Embedding icon from assets/off.ico ===
:: We use the rsrc tool to create a resource file (.syso) from the .ico icon.
:: The Go linker automatically picks up this .syso file.
rsrc -ico assets/app.ico -manifest app.manifest -o rsrc.syso
if %ERRORLEVEL% NEQ 0 (
    echo !!! Failed to embed icon. Skipping... !!!
)
:: === END ADDED ===

:: Определяем имя выходного файла с авто-нумерацией
echo.
echo === Determining output filename ===
set "BASE_NAME=singbox-launcher"
set "EXTENSION=.exe"
set "OUTPUT_FILENAME=%BASE_NAME%%EXTENSION%"
set "COUNTER=0"

:FIND_UNIQUE_FILENAME
if exist "%OUTPUT_FILENAME%" (
    set /a COUNTER+=1
    set "OUTPUT_FILENAME=%BASE_NAME%-!COUNTER!%EXTENSION%"
    goto :FIND_UNIQUE_FILENAME
)

echo Using output file: "%OUTPUT_FILENAME%"

:: Собираем проект
echo.
echo === Starting Build ===
go build -ldflags="-H windowsgui" -o "%OUTPUT_FILENAME%"

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo !!! Build failed !!!
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo === Build completed successfully ===
pause