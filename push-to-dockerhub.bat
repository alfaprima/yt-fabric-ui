@echo off
REM Script to build and push Docker image to DockerHub
REM Usage: push-to-dockerhub.bat [version]

setlocal enabledelayedexpansion

REM Configuration
set DOCKERHUB_USERNAME=alfaprima
set IMAGE_NAME=yt-fabric-ui

REM Get version from argument or docker-compose.yml
if "%~1"=="" (
    REM Extract version from docker-compose.yml
    for /f "tokens=2 delims=:" %%a in ('findstr /r "image: %IMAGE_NAME%:" docker-compose.yml') do (
        set VERSION=%%a
        set VERSION=!VERSION:~0,-1!
        set VERSION=!VERSION: =!
    )
    if "!VERSION!"=="" (
        echo Error: Could not extract version from docker-compose.yml
        echo Usage: %~nx0 [version]
        exit /b 1
    )
) else (
    set VERSION=%~1
)

echo ==========================================
echo Building and pushing Docker image
echo Username: %DOCKERHUB_USERNAME%
echo Image: %IMAGE_NAME%
echo Version: %VERSION%
echo ==========================================

REM Build the Docker image with both version and latest tags
echo.
echo Step 1: Building Docker image...
docker build -t %DOCKERHUB_USERNAME%/%IMAGE_NAME%:%VERSION% -t %DOCKERHUB_USERNAME%/%IMAGE_NAME%:latest .
if errorlevel 1 (
    echo Error: Docker build failed
    exit /b 1
)

REM Check if logged in to DockerHub
echo.
echo Step 2: Checking DockerHub login...
docker info | findstr /C:"Username: %DOCKERHUB_USERNAME%" >nul
if errorlevel 1 (
    echo Not logged in to DockerHub. Attempting login...
    docker login
    if errorlevel 1 (
        echo Error: Docker login failed
        exit /b 1
    )
) else (
    echo Already logged in to DockerHub as %DOCKERHUB_USERNAME%
)

REM Push the versioned tag
echo.
echo Step 3: Pushing version %VERSION%...
docker push %DOCKERHUB_USERNAME%/%IMAGE_NAME%:%VERSION%
if errorlevel 1 (
    echo Error: Failed to push version %VERSION%
    exit /b 1
)

REM Push the latest tag
echo.
echo Step 4: Pushing latest tag...
docker push %DOCKERHUB_USERNAME%/%IMAGE_NAME%:latest
if errorlevel 1 (
    echo Error: Failed to push latest tag
    exit /b 1
)

echo.
echo ==========================================
echo Successfully pushed to DockerHub!
echo   - %DOCKERHUB_USERNAME%/%IMAGE_NAME%:%VERSION%
echo   - %DOCKERHUB_USERNAME%/%IMAGE_NAME%:latest
echo ==========================================

endlocal
