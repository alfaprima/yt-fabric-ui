# Docker Setup for YouTube Fabric UI

This document provides instructions for running the YouTube Fabric UI application in a Docker container.

## Prerequisites

- Docker installed on your system
- Docker Compose (optional, but recommended)

## Building and Running with Docker

### Option 1: Using Docker Compose (Recommended)

0. Create your runtime env file for the container:
   ```bash
   cp .env.example .env
   ```

   Required auth values in `.env`:
   ```env
   AUTH_ENABLED=true
   AUTH_BOOTSTRAP_ADMIN_USER=admin
   AUTH_BOOTSTRAP_ADMIN_PASS=use-a-long-random-password
   AUTH_COOKIE_SECURE=true
   FABRIC_ENV_EDIT_ENABLED=false
   ```

   `FABRIC_ENV_EDIT_ENABLED=false` is recommended for production. This disables editing `~/.config/fabric/.env` from the UI.

   Optional global model parameters:
   ```env
   # FABRIC_GLOBAL_TEMPERATURE=1
   # FABRIC_GLOBAL_TOP_P=0.9
   # FABRIC_GLOBAL_PRESENCE_PENALTY=0
   # FABRIC_GLOBAL_FREQUENCY_PENALTY=0
   # FABRIC_GLOBAL_RAW=true
   ```

   For GPT-5/o1/o3/o4 model families, set `FABRIC_GLOBAL_RAW=true` to avoid unsupported custom temperature errors.

1. Build and start the container:
   ```bash
   docker-compose up
   ```

   This will build the Docker image and start the container. The application will be accessible at http://localhost:8090.

2. To run in detached mode (in the background):
   ```bash
   docker-compose up -d
   ```

3. To stop the container:
   ```bash
   docker-compose down
   ```

### Option 2: Using Docker Commands

1. Build the Docker image:
   ```bash
   docker build -t yt-fabric-ui .
   ```

2. Run the container:
   ```bash
   docker run -p 8090:8090 -v $(pwd)/data:/app/data yt-fabric-ui
   ```

   The application will be accessible at http://localhost:8090.

## Data Persistence

The application stores video data in the `data/videos` directory. This directory is mounted as a volume in the Docker container, ensuring that your data persists across container restarts.

## Configuration

You can configure the application by modifying the environment variables in the `docker-compose.yml` file or by passing them to the `docker run` command.

For example, to change the port:

```bash
docker run -p 9090:8090 -v $(pwd)/data:/app/data -e PORT=8090 yt-fabric-ui
```

## Troubleshooting

If you encounter any issues:

1. Check the container logs:
   ```bash
   docker-compose logs
   ```
   or
   ```bash
   docker logs <container_id>
   ```

2. Ensure the `data` directory has the correct permissions.

3. Verify that the fabric CLI tool is installed correctly in the container:
   ```bash
   docker exec -it <container_id> fabric -h
   ```
