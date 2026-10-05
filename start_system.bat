@echo off
cd /d "%~dp0"

echo =========================================================================
echo   College Admission Stats System (110-115 Academic Years)
echo =========================================================================
echo [INFO] Current Directory: %~dp0
echo [INFO] Target Port: 8063
echo.

echo [1/4] Stopping existing app processes and freeing port 8063...
taskkill /F /IM app.exe >nul 2>&1
for /f "tokens=5" %%a in ('netstat -aon 2^>nul ^| findstr ":8063 " ^| findstr "LISTENING"') do taskkill /F /PID %%a >nul 2>&1
timeout /t 1 /nobreak >nul

echo [2/4] Checking build status...
if exist "cmd\server\main.go" (
    echo [INFO] Compiling latest Go server build...
    go build -o app.exe ./cmd/server
)

if not exist "app.exe" (
    echo [ERROR] app.exe not found and build failed.
    echo Please make sure Go is installed and in your system PATH.
    pause
    exit /b 1
)

echo [3/4] Launching default browser in 2 seconds (http://localhost:8063)...
start "" cmd /c "timeout /t 2 /nobreak >nul & start http://localhost:8063"

echo [4/4] Starting server on http://localhost:8063...
echo =========================================================================
echo   Server is running. Keep this window open. Press Ctrl+C to stop.
echo =========================================================================
echo.

app.exe

if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Server exited with error code %errorlevel%.
    pause
)
