@echo off
setlocal
cd /d "%~dp0"

for /f %%i in ('git describe --tags --always 2^>nul') do set VERSION=%%i
if not defined VERSION set VERSION=dev
for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=none
for /f %%i in ('powershell -NoProfile -Command "(Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')"') do set BUILD_TIME=%%i

set LDFLAGS=-s -w -X main.version=%VERSION% -X main.commit=%COMMIT% -X main.buildTime=%BUILD_TIME%

if not exist dist mkdir dist

echo ==^> windows/amd64
set GOOS=windows
set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o dist\dof-admin-windows-amd64.exe .\cmd\admin || goto :fail

echo ==^> linux/amd64
set GOOS=linux
set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o dist\dof-admin-linux-amd64 .\cmd\admin || goto :fail

echo ==^> linux/arm64
set GOOS=linux
set GOARCH=arm64
go build -ldflags "%LDFLAGS%" -o dist\dof-admin-linux-arm64 .\cmd\admin || goto :fail

dir dist
endlocal
exit /b 0

:fail
echo build failed
endlocal
exit /b 1
