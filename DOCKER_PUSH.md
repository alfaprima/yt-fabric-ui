# Docker Push Scripts

This directory contains scripts to automate building and pushing Docker images to DockerHub.

## Available Scripts

### For Linux/Mac: `push-to-dockerhub.sh`
### For Windows: `push-to-dockerhub.bat`

## Features

- Automatically builds Docker image with multi-stage build
- Tags image with both version number and `latest` tag
- Uses explicit Docker Hub login (`docker login --username ...`)
- Pushes both tags to DockerHub
- Requires explicit version argument (safer and predictable)
- Pulls newest base images during build (`docker build --pull`)

## Usage

### Push a Version

You must specify a version as an argument:

**Linux/Mac:**
```bash
./push-to-dockerhub.sh 1.0.7
```

**Windows:**
```cmd
push-to-dockerhub.bat 1.0.7
```

## What the Scripts Do

1. **Validate Version**: Uses a required CLI version argument
2. **Build Image**: Builds Docker image with both version and `latest` tags
   - `alfaprima/yt-fabric-ui:X.X.X`
   - `alfaprima/yt-fabric-ui:latest`
3. **Login**: Authenticates to Docker Hub
4. **Push Images**: Pushes both tags to DockerHub

## Prerequisites

- Docker installed and running
- DockerHub account credentials
- Prefer Docker Hub Personal Access Token over password

## Configuration

To change the DockerHub username or image name, edit the configuration section in the script:

**Bash script (`push-to-dockerhub.sh`):**
```bash
DOCKERHUB_USERNAME="alfaprima"
IMAGE_NAME="yt-fabric-ui"
```

**Batch script (`push-to-dockerhub.bat`):**
```batch
set DOCKERHUB_USERNAME=alfaprima
set IMAGE_NAME=yt-fabric-ui
```

## Example Output

```
==========================================
Building and pushing Docker image
Username: alfaprima
Image: yt-fabric-ui
Version: 1.0.6
==========================================

Step 1: Building Docker image...
[+] Building 0.4s (21/21) FINISHED
...

Step 2: DockerHub login...
Login Succeeded

Step 3: Pushing version 1.0.6...
The push refers to repository [docker.io/alfaprima/yt-fabric-ui]
...
1.0.6: digest: sha256:... size: 856

Step 4: Pushing latest tag...
...
latest: digest: sha256:... size: 856

==========================================
✓ Successfully pushed to DockerHub!
  - alfaprima/yt-fabric-ui:1.0.6
  - alfaprima/yt-fabric-ui:latest
==========================================
```

## Troubleshooting

### Not Logged In
If you're not logged in to DockerHub, the script will prompt you:
```bash
docker login --username alfaprima
```

### Build Fails
Ensure your Dockerfile is valid and all dependencies are available.

### Push Fails
- Check your internet connection
- Verify DockerHub credentials
- Ensure your account can push to `alfaprima/yt-fabric-ui`

## Updating Version

To push a new version, run:

```bash
./push-to-dockerhub.sh 1.0.7
```
