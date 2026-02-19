# Troubleshooting Guide

## Common Issues and Solutions

### Error: "temperature does not support 0.7 with this model"

**Problem:** When using certain AI models (like GPT-5, O1, O3, O4 series), you may encounter an error:
```
error, status code: 400, status: 400 Bad Request, message: Unsupported value: 'temperature' does not support 0.7 with this model. Only the default (1) value is supported.
```

**Cause:** OpenAI's reasoning models (GPT-5, O1, O3, O4 series) do not support custom temperature parameters. These models only work with their default temperature setting of 1.0.

**Solution:** Use a different model that supports temperature customization. Recommended alternatives:

- **GPT-4o** - Latest GPT-4 optimized model
- **GPT-4-turbo** - Fast and capable
- **GPT-4** - Standard GPT-4 model
- **Claude models** - Anthropic's Claude 3.5 Sonnet or Haiku
- **Groq models** - Fast inference with Llama models

Or force fabric raw mode globally from the app container:

```env
FABRIC_GLOBAL_RAW=true
```

This makes the app call fabric with `--raw` (no custom temperature/top_p flags).

**How to change the model:**

1. In the web UI, select a different model from the dropdown when processing a video
2. Or update your `.config/fabric/.env` file to change the default model:
   ```
   DEFAULT_MODEL=gpt-4o
   ```

**Models that DON'T support temperature:**
- gpt-5 (all variants)
- o1, o1-mini, o1-pro
- o3, o3-mini
- o4-mini

**Models that DO support temperature:**
- gpt-4o, gpt-4o-mini
- gpt-4, gpt-4-turbo
- gpt-3.5-turbo
- claude-3-5-sonnet, claude-3-5-haiku
- All Groq/Llama models

### Error: "fabric: error executing fabric pattern: exit status 1"

**Problem:** Generic fabric execution error without details.

**Cause:** This can be caused by:
1. Missing or invalid API keys
2. Model incompatibility (see temperature issue above)
3. Network connectivity issues
4. Invalid pattern configuration

**Solution:**

1. **Check your API keys** in `.config/fabric/.env`:
   ```bash
   cat .config/fabric/.env
   ```
   Ensure your `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GROQ_API_KEY` are valid.

2. **Test fabric directly** in the container:
   ```bash
   docker exec -it $(docker ps -q -f name=yt-fabric-ui) sh
   echo "test" | fabric --pattern summarize --model gpt-4o
   ```

3. **Check the Process.log** file for detailed error messages:
   ```bash
   docker exec $(docker ps -q -f name=yt-fabric-ui) cat /app/Process.log
   ```

4. **Inspect per-video trace file** for request details (service URL, payload summary, full stderr):
   ```bash
   ls -la ./data/videos/<video_id>/llm-trace-*.log
   ```

5. **Try a different model** - Switch from reasoning models (gpt-5, o1) to standard models (gpt-4o, claude-3-5-sonnet)

### Docker Container Issues

**Problem:** Container fails to start or fabric commands don't work.

**Solution:**

1. **Ensure fabric configuration is mounted correctly:**
   ```bash
   docker-compose down
   docker-compose up -d
   ```

2. **Verify the configuration directory exists:**
   ```bash
   ls -la .config/fabric/
   ```
   Should contain `.env` file and `patterns/` directory.

3. **Check container logs:**
   ```bash
   docker-compose logs -f
   ```

4. **Rebuild the container if needed:**
   ```bash
   docker-compose down
   docker-compose build --no-cache
   docker-compose up -d
   ```

### API Key Issues

**Problem:** Authentication errors or "invalid API key" messages.

**Solution:**

1. **Verify your API keys are valid** by testing them directly:
   ```bash
   # Test OpenAI key
   curl https://api.openai.com/v1/models \
     -H "Authorization: Bearer YOUR_API_KEY"
   ```

2. **Update your `.config/fabric/.env` file** with valid keys:
   ```
   OPENAI_API_KEY=sk-...
   ANTHROPIC_API_KEY=sk-ant-...
   GROQ_API_KEY=gsk_...
   ```

3. **Restart the container** after updating keys:
   ```bash
   docker-compose restart
   ```

### Pattern Not Found

**Problem:** Error indicating a pattern doesn't exist.

**Solution:**

1. **List available patterns:**
   ```bash
   docker exec $(docker ps -q -f name=yt-fabric-ui) fabric -l
   ```

2. **Update patterns** from the fabric repository:
   ```bash
   docker exec $(docker ps -q -f name=yt-fabric-ui) fabric --updatepatterns
   ```

## Getting Help

If you continue to experience issues:

1. Check the `Process.log` file for detailed error messages
2. Review the container logs: `docker-compose logs`
3. Ensure your API keys are valid and have sufficient credits
4. Try using a different AI model
5. Open an issue on the GitHub repository with:
   - Error message
   - Model being used
   - Pattern being used
   - Relevant log entries
