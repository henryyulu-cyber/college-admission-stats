@echo off
setlocal

pushd "%~dp0"
set "EXE_NAME=app.exe"

echo ===================================================
echo   College Admission Stats System - Shutdown
echo ===================================================

echo [INFO] Terminating running application instances...
taskkill /F /IM "%EXE_NAME%" >nul 2>&1

if errorlevel 1 (
    echo [INFO] No active process named %EXE_NAME% was found.
) else (
    echo [SUCCESS] Application %EXE_NAME% stopped successfully.
)

echo ===================================================
pause
