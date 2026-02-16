# Example Plugins

This directory contains template and example plugins for documentation purposes.

## Creating a New Plugin

### 1. Copy the Template

```bash
# For a new format plugin
cp -r plugins/example/template plugins/format/myformat

# For a new tool plugin
cp -r plugins/example/template plugins/tool/mytool

# For a new custom kind (after adding to PluginKinds)
cp -r plugins/example/template plugins/mykind/myplugin
```

### 2. Update plugin.json

Edit `plugin.json` and:

- Remove all `_comment_*` and `_*_help` fields
- Set `plugin_id` to `<kind>.<name>` (e.g., `format.myformat`)
- Set `kind` to match the parent directory (`format`, `tool`, etc.)
- Set `entrypoint` to your executable name (e.g., `format-myformat`)
- Configure `capabilities` and `ir_support` as needed

Example minimal plugin.json:
```json
{
  "plugin_id": "format.myformat",
  "version": "1.0.0",
  "kind": "format",
  "entrypoint": "format-myformat"
}
```

### 3. Create the Executable

Create your plugin executable (in Go, typically `main.go`):

```go
package main

import (
    "encoding/json"
    "os"
)

func main() {
    // Read IPC request from stdin
    var req map[string]interface{}
    json.NewDecoder(os.Stdin).Decode(&req)

    // Handle command
    switch req["command"] {
    case "detect":
        // Handle detect command
    case "ingest":
        // Handle ingest command
    }

    // Write response to stdout
    json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
        "status": "ok",
        "result": map[string]interface{}{},
    })
}
```

Build:
```bash
go build -o plugins/format/myformat/format-myformat ./plugins/format/myformat
```

### 4. Test Your Plugin

```bash
# Test detection
echo '{"command":"detect","args":{"path":"testfile.txt"}}' | ./plugins/format/myformat/format-myformat

# List all plugins
./capsule plugins
```

## Adding a New Plugin Kind

To add an entirely new plugin kind (not format/tool/juniper/example):

1. **Update loader.go** - Add to `PluginKinds` slice:
   ```go
   var PluginKinds = []string{"format", "tool", "juniper", "example", "mykind"}
   ```

2. **Add helper method** on Plugin:
   ```go
   func (p *Plugin) IsMyKind() bool {
       return p.Manifest.Kind == "mykind"
   }
   ```

3. **Create directory**:
   ```bash
   mkdir -p plugins/mykind
   ```

4. **Add test** in `loader_test.go` (see `TestExamplePluginKind`)

5. **Update documentation** as needed

See `core/plugins/loader.go` for detailed comments on the process.
