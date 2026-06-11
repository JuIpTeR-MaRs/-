@echo off
title Dormitory Repair System - Startup Script
echo ===================================================
echo    Dormitory Repair System - Startup Launcher
echo ===================================================
echo.

:: 1. Check MySQL Port
echo [1/3] Checking local MySQL status...
netstat -ano | findstr :3306 > nul
if %errorlevel% neq 0 (
    echo [WARNING] Local MySQL service on port 3306 not detected! Please ensure MySQL is running.
) else (
    echo [ OK ] MySQL port 3306 is active.
)

:: 2. Check Redis Port
echo [2/3] Checking local Redis status...
netstat -ano | findstr :6379 > nul
if %errorlevel% neq 0 (
    echo [WARNING] Local Redis service on port 6379 not detected! Please ensure Redis is running.
) else (
    echo [ OK ] Redis port 6379 is active.
)
echo.

:: 3. Run Server
echo [3/3] Starting backend server...
set "RUN_CMD=go run cmd/server/main.go"

if exist "server.exe" (
    echo ---------------------------------------------------
    echo Detected precompiled server.exe, running directly...
    echo ---------------------------------------------------
    set "RUN_CMD=server.exe"
) else (
    echo ---------------------------------------------------
    echo server.exe not found, compiling from Go source...
    echo ---------------------------------------------------
)

:: Execute startup command outside of parenthesis to prevent premature terminal termination
%RUN_CMD%

echo.
echo Server exited.
pause
