# Noop Plugin - SDK Example

This is a placeholder plugin demonstrating the plugin SDK pattern.

## Dual Implementation Pattern

This plugin has two implementations controlled by build tags:

1. **main.go** (`//go:build !sdk`) - Direct IPC implementation
   - Uses raw JSON encoding/decoding with stdin/stdout
   - Manually handles the IPC protocol
   - Suitable for minimal plugins or custom protocols

2. **main_sdk.go** (`//go:build sdk`) - SDK-based implementation
   - Uses the format SDK package for standardized plugin structure
   - Demonstrates proper use of `ir.NewCorpus()` with required arguments
   - Shows the recommended pattern for format plugins
   - Implements all standard format plugin methods: `detect`, `parse`, `emit`, `enumerate`

## Build Tags

The build tag system ensures only one implementation is compiled:

- Default build (`go build`): Compiles `main.go` (direct IPC)
- SDK build (`go build -tags=sdk`): Compiles `main_sdk.go` (SDK pattern)

## SDK Pattern Demonstration

The SDK version shows how to properly use the format SDK:

```go
// Creating a corpus with proper arguments
ir.NewCorpus("noop", "placeholder", "0.0.0")
// Arguments: format, source, version
```

This plugin serves as a reference implementation for external plugin developers.

## Built-in Plugins

Core plugin functionality is embedded directly into the main binaries (`capsule`, `capsule-web`, `capsule-api`). No external plugins are required for standard operation.

## Adding External Plugins

To add external plugins:

1. Create a subdirectory under `format/`, `tool/`, or the appropriate kind directory
2. Include a `plugin.json` manifest file
3. Include the compiled plugin binary

Example structure:
```
plugins/
├── format/
│   └── my-plugin/
│       ├── plugin.json
│       └── format-my-plugin
└── tool/
    └── my-tool/
        ├── plugin.json
        └── tool-my-tool
```

## Enabling External Plugins

External plugins are disabled by default. To enable them, use the `--plugins-external` flag or set the `CAPSULE_PLUGINS_EXTERNAL=1` environment variable.

## Plugin Development

See the main documentation for plugin development guidelines.
