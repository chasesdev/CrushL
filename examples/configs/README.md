# Crush Configuration Examples

This directory contains example configurations for different LLM providers.

## LM Studio Configuration

The `lm_studio.json` file demonstrates how to configure Crush to work with [LM Studio](https://lmstudio.ai/) for local model hosting.

### Features

1. **Local Model Hosting**: Run models on your own hardware via LM Studio
2. **Reasoning Layer**: Automatic task management using semantic analysis
3. **Model Auto-Detection**: Automatically selects the best available model

### Configuration Sections

#### Providers

```json
"providers": {
  "lmstudio": {
    "name": "LM Studio",
    "base_url": "http://localhost:1234/v1/",
    "type": "openai"
  }
}
```

Configures the LM Studio provider with OpenAI-compatible API endpoint.

#### Models

```json
"models": {
  "large": {
    "model": "qwen3-8b",
    "provider": "lmstudio"
  },
  "small": {
    "model": "qwen3-8b",
    "provider": "lmstudio"
  }
}
```

Maps model types (large/small) to specific models in LM Studio.

#### Reasoning Layer

```json
"reasoning": {
  "enabled": true,
  "provider": "lmstudio",
  "base_url": "http://localhost:1234",
  "model_preference": ["glm-4.6", "qwen3-next-80b", "qwen3-8b"],
  "fallback_model": "qwen3-8b",
  "auto_create_tasks": true,
  "auto_update_tasks": true
}
```

The reasoning layer provides intelligent task management:

- **enabled**: Enable/disable the reasoning layer
- **provider**: Provider ID to use for reasoning (must match a configured provider)
- **base_url**: Base URL for model detection (queries /v1/models endpoint)
- **model_preference**: Ordered list of preferred models (tries in order)
- **fallback_model**: Model to use if none of the preferred models are available
- **auto_create_tasks**: Automatically extract and create tasks from user requests
- **auto_update_tasks**: Automatically update task status based on LLM responses

##### How It Works

**Pre-Processing (User Request Analysis)**:
1. When you send a request, the reasoning layer analyzes it
2. Extracts discrete, actionable tasks
3. Stores tasks in `.crush/sessions/{sessionID}/tasks.json`
4. Assigns priority levels (high/medium/low)

**Post-Processing (Response Analysis)**:
1. Analyzes the assistant's response
2. Marks tasks as completed when work is done
3. Creates new tasks for follow-up work discovered
4. Updates task statuses (pending → in_progress → completed)

**Model Priority**:
The reasoning layer auto-detects loaded models and selects the best one:
1. **GLM-4.6** (best for complex reasoning)
2. **Qwen3-Next-80B** (high-end model)
3. **Qwen3-8B** (standard model, good balance)

If none are available, falls back to `fallback_model`.

**Task Storage**:
Tasks are stored per session in:
```
.crush/
  sessions/
    {session-id}/
      tasks.json
```

Each task includes:
- ID, description, status, priority
- Creation, update, and completion timestamps
- Tags for categorization
- Support for subtasks

### Options

```json
"options": {
  "debug": false,
  "debug_lsp": false,
  "data_directory": ".crush"
}
```

- **debug**: Enable debug logging
- **debug_lsp**: Enable LSP debug logging
- **data_directory**: Directory for storing session data and tasks

### Permissions

```json
"permissions": {
  "allowed_tools": ["view", "ls", "grep", "edit", "bash", "write"]
}
```

Controls which tools are available without permission prompts.

## Setup Instructions

### 1. Install LM Studio

Download and install from [lmstudio.ai](https://lmstudio.ai/)

### 2. Load a Model

In LM Studio, download and load one of these recommended models:
- **Qwen3-8B**: Good all-around model
- **Qwen3-Next-80B**: High-end reasoning (requires more VRAM)
- **GLM-4.6**: Excellent for complex reasoning tasks

### 3. Start the Server

In LM Studio:
1. Go to "Local Server" tab
2. Click "Start Server"
3. Verify it's running on `http://localhost:1234`

### 4. Copy Configuration

```bash
# Copy the example config to your project
cp examples/configs/lm_studio.json .crush/config.json

# Or create a project-specific config
mkdir -p .crush
cp examples/configs/lm_studio.json .crush/config.json
```

### 5. Run Crush

```bash
crush
```

The reasoning layer will:
- Auto-detect your loaded model
- Analyze your requests and create tasks
- Track progress automatically
- Update task statuses based on work completed

## Example Session

```
You: Help me refactor the authentication module and add tests

[Reasoning Layer creates tasks:]
- Task 1: Analyze current authentication module (high priority)
- Task 2: Refactor authentication code (high priority)
- Task 3: Write unit tests for authentication (medium priority)
- Task 4: Write integration tests (medium priority)

[As work progresses, tasks automatically update:]
✓ Task 1: Completed
→ Task 2: In progress
⋯ Task 3: Pending
⋯ Task 4: Pending
```

## Advanced Configuration

### Custom Model Preferences

Adjust `model_preference` to match your setup:

```json
"model_preference": [
  "my-custom-model-v2",
  "qwen3-8b"
]
```

### Disable Auto-Features

Turn off auto-creation or auto-update if you prefer manual control:

```json
"reasoning": {
  "auto_create_tasks": false,
  "auto_update_tasks": true
}
```

### Use Different Provider for Reasoning

You can use a different provider for reasoning vs. main chat:

```json
"reasoning": {
  "provider": "openai",  // Use OpenAI for reasoning
  "base_url": "https://api.openai.com/v1"
}
```

## Troubleshooting

### Reasoning Layer Not Working

1. Check LM Studio is running: `curl http://localhost:1234/v1/models`
2. Enable debug logging: `"debug": true` in options
3. Verify model is loaded in LM Studio
4. Check logs for reasoning layer initialization

### Model Detection Issues

If auto-detection fails, check:
1. Base URL is correct
2. Model is actually loaded in LM Studio
3. Fallback model is available

### Task Storage Issues

Tasks are stored in `.crush/sessions/{sessionID}/tasks.json`. Check:
1. Directory permissions
2. Disk space
3. File system access

## See Also

- [Crush Documentation](https://github.com/charmbracelet/crush)
- [LM Studio Documentation](https://lmstudio.ai/docs)
- [Model Recommendations](https://github.com/charmbracelet/crush/wiki/models)
