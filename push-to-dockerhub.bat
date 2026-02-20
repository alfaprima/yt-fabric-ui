@echo off
REM Script to build and push Docker image to DockerHub
REM Usage: push-to-dockerhub.bat <version>

setlocal enabledelayedexpansion

REM Configuration
set DOCKERHUB_USERNAME=alfaprima
set IMAGE_NAME=yt-fabric-ui
set FULL_IMAGE=%DOCKERHUB_USERNAME%/%IMAGE_NAME%

REM Get version from argument
if "%~1"=="" (
    echo Usage: %~nx0 ^<version^>
    echo Example: %~nx0 1.0.7
    exit /b 1
) else (
    set VERSION=%~1
)

echo %VERSION% | findstr /R "^[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\([.-][0-9A-Za-z.-][0-9A-Za-z.-]*\)*$" >nul
if errorlevel 1 (
    echo Error: Version must look like semver (e.g. 1.0.7 or 1.0.7-rc1)
    exit /b 1
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
docker build --pull -t %FULL_IMAGE%:%VERSION% -t %FULL_IMAGE%:latest .
if errorlevel 1 (
    echo Error: Docker build failed
    exit /b 1
)

REM Ensure logged in to DockerHub (use access token if possible)
echo.
echo Step 2: DockerHub login...
docker login --username %DOCKERHUB_USERNAME%
if errorlevel 1 (
    echo Error: Docker login failed
    exit /b 1
)

REM Push the versioned tag
echo.
echo Step 3: Pushing version %VERSION%...
docker push %FULL_IMAGE%:%VERSION%
if errorlevel 1 (
    echo Error: Failed to push version %VERSION%
    exit /b 1
)

REM Push the latest tag
echo.
echo Step 4: Pushing latest tag...
docker push %FULL_IMAGE%:latest
if errorlevel 1 (
    echo Error: Failed to push latest tag
    exit /b 1
)

echo.
echo ==========================================
echo Successfully pushed to DockerHub!
echo   - %FULL_IMAGE%:%VERSION%
echo   - %FULL_IMAGE%:latest
echo ==========================================

endlocal
