Based on your current Docker configuration, here's a step-by-step guide to deploy your yt-fabric-ui application on a Portainer-managed server:

## Prerequisites on Target Server
- Portainer installed and running
- Docker installed
- Network access to the server

## Step-by-Step Deployment Instructions

### Step 1: Prepare the Docker Image
You have two options:

**Option A: Build and Push to a Registry (Recommended)**
1. Build the image locally:
   ```
   docker build -t your-registry/yt-fabric-ui:1.0.4 .
   ```
2. Push to Docker Hub or your private registry:
   ```
   docker login
   docker push your-registry/yt-fabric-ui:1.0.4
   ```

**Option B: Export/Import Image**
1. Build the image locally:
   ```
   docker build -t yt-fabric-ui:1.0.4 .
   ```
2. Save the image to a tar file:
   ```
   docker save -o yt-fabric-ui-1.0.4.tar yt-fabric-ui:1.0.4
   ```
3. Transfer the tar file to your target server (via SCP, FTP, etc.)
4. On the target server, load the image:
   ```
   docker load -i yt-fabric-ui-1.0.4.tar
   ```

### Step 2: Prepare Required Directories on Target Server
Create the necessary directories on your target server:
```
mkdir -p /path/to/data
mkdir -p /path/to/.config/fabric
```

### Step 3: Configure Fabric on Target Server
The application requires fabric configuration. Copy your local fabric config to the target server:
```
# From your local machine, copy the fabric config
scp -r ./.config/fabric user@target-server:/path/to/.config/fabric
```

### Step 4: Deploy via Portainer

**Using Portainer Stacks (Recommended):**
1. Log into Portainer web interface
2. Navigate to **Stacks** → **Add stack**
3. Name your stack (e.g., "yt-fabric-ui")
4. Choose **Web editor** and paste this docker-compose configuration:

```yaml
version: '3.8'
services:
  yt-fabric-ui:
    image: yt-fabric-ui:1.0.4  # Or your-registry/yt-fabric-ui:1.0.4 if using a registry
    ports:
      - "8090:8090"
    volumes:
      - /path/to/data:/app/data
      - /path/to/.config/fabric:/home/fabric/.config/fabric
    environment:
      - PORT=8090
    restart: unless-stopped
```

5. Update the volume paths to match your target server paths
6. Click **Deploy the stack**

**Alternative: Using Portainer Containers:**
1. Navigate to **Containers** → **Add container**
2. Fill in the details:
   - **Name**: yt-fabric-ui
   - **Image**: yt-fabric-ui:1.0.4 (or your registry path)
   - **Port mapping**: Host 8090 → Container 8090
   - **Volumes**:
     - `/path/to/data` → `/app/data`
     - `/path/to/.config/fabric` → `/home/fabric/.config/fabric`
   - **Environment variables**:
     - PORT=8090
   - **Restart policy**: Unless stopped
3. Click **Deploy the container**

### Step 5: Verify Deployment
1. In Portainer, check the container logs to ensure it started successfully
2. Access the application at: `http://target-server-ip:8090`

## Important Configuration Notes

**Required Volumes:**
- `/app/data` - Stores video data and processing results
- `/home/fabric/.config/fabric` - Contains fabric AI framework configuration (API keys, patterns)

**Port:**
- Default: 8090 (can be changed via PORT environment variable)

**User Permissions:**
- Container runs as user `fabric` (UID 1000)
- Ensure volume directories have appropriate permissions

**Fabric Configuration:**
- The fabric config directory must contain your API keys and patterns
- Without proper fabric setup, the AI processing features won't work

## Troubleshooting

If the container fails to start:
1. Check Portainer logs for the container
2. Verify volume paths exist and have correct permissions
3. Ensure fabric configuration is properly copied
4. Verify port 8090 is not already in use on the target server

If using a registry and image pull fails:
1. Ensure the target server can access your registry
2. Configure registry credentials in Portainer under **Registries**