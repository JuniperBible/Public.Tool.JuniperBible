# Juniper Bible - Developer Integration Guide

This guide shows how to integrate Juniper Bible into your applications by wrapping the CLI.

## Overview

Juniper Bible provides a CLI that can be easily wrapped in any programming language. The CLI outputs structured information and uses standard exit codes, making it ideal for programmatic use.

## Python Integration

### Basic Python Wrapper

```python
import subprocess
import json
from pathlib import Path
from typing import Optional, Dict, Any

class CapsuleCLI:
    """Python wrapper for Juniper Bible CLI."""

    def __init__(self, binary_path: str = "./capsule"):
        self.binary = binary_path

    def _run(self, *args, capture_output: bool = True) -> subprocess.CompletedProcess:
        """Execute capsule command and return result."""
        return subprocess.run(
            [self.binary, *args],
            capture_output=capture_output,
            text=True
        )

    def convert(self, input_path: str, to_format: str, output_path: str) -> str:
        """Convert file to target format via IR.

        Args:
            input_path: Path to input file
            to_format: Target format (osis, epub, sqlite, etc.)
            output_path: Path for output file

        Returns:
            Path to output file

        Raises:
            Exception: If conversion fails
        """
        result = self._run("convert", input_path, "--to", to_format, "--out", output_path)
        if result.returncode != 0:
            raise Exception(f"Conversion failed: {result.stderr}")
        return output_path

    def extract_ir(self, input_path: str, output_path: str) -> str:
        """Extract Intermediate Representation from file.

        Args:
            input_path: Path to source file
            output_path: Path for IR JSON output

        Returns:
            Path to IR file
        """
        result = self._run("extract-ir", input_path, "--out", output_path)
        if result.returncode != 0:
            raise Exception(f"IR extraction failed: {result.stderr}")
        return output_path

    def emit_native(self, ir_path: str, format_name: str, output_path: str) -> str:
        """Generate native format from IR.

        Args:
            ir_path: Path to IR JSON file
            format_name: Target format name
            output_path: Path for output file

        Returns:
            Path to output file
        """
        result = self._run("emit-native", ir_path, "--format", format_name, "--out", output_path)
        if result.returncode != 0:
            raise Exception(f"Emit failed: {result.stderr}")
        return output_path

    def detect_format(self, file_path: str) -> str:
        """Detect file format.

        Args:
            file_path: Path to file

        Returns:
            Detected format name (e.g., "format.osis")
        """
        result = self._run("detect", file_path)
        return result.stdout.strip()

    def list_plugins(self) -> str:
        """List available plugins.

        Returns:
            Plugin listing as string
        """
        result = self._run("plugins")
        return result.stdout

    def ingest(self, input_path: str, output_path: str) -> str:
        """Ingest file into capsule.

        Args:
            input_path: Path to file or directory
            output_path: Path for output capsule

        Returns:
            Path to capsule file
        """
        result = self._run("ingest", input_path, "--out", output_path)
        if result.returncode != 0:
            raise Exception(f"Ingest failed: {result.stderr}")
        return output_path

    def verify(self, capsule_path: str) -> bool:
        """Verify capsule integrity.

        Args:
            capsule_path: Path to capsule file

        Returns:
            True if verification passes
        """
        result = self._run("verify", capsule_path)
        return result.returncode == 0

    def ir_info(self, ir_path: str) -> Dict[str, Any]:
        """Get IR structure information.

        Args:
            ir_path: Path to IR JSON file

        Returns:
            Dictionary with IR metadata
        """
        result = self._run("ir-info", ir_path, "--json")
        if result.returncode != 0:
            raise Exception(f"IR info failed: {result.stderr}")
        return json.loads(result.stdout)


# Usage example
if __name__ == "__main__":
    cli = CapsuleCLI("./capsule")

    # Convert USFM to OSIS
    cli.convert("input.usfm", "osis", "output.osis")

    # Batch convert
    from pathlib import Path
    for usfm_file in Path(".").glob("*.usfm"):
        output = usfm_file.with_suffix(".osis")
        cli.convert(str(usfm_file), "osis", str(output))
```

### Async Python Wrapper

```python
import asyncio
import subprocess
from typing import Optional

class AsyncCapsuleCLI:
    """Async Python wrapper for Juniper Bible CLI."""

    def __init__(self, binary_path: str = "./capsule"):
        self.binary = binary_path

    async def _run(self, *args) -> asyncio.subprocess.Process:
        """Execute capsule command asynchronously."""
        proc = await asyncio.create_subprocess_exec(
            self.binary, *args,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE
        )
        return proc

    async def convert(self, input_path: str, to_format: str, output_path: str) -> str:
        """Convert file asynchronously."""
        proc = await self._run("convert", input_path, "--to", to_format, "--out", output_path)
        stdout, stderr = await proc.communicate()
        if proc.returncode != 0:
            raise Exception(f"Conversion failed: {stderr.decode()}")
        return output_path

    async def batch_convert(self, files: list, to_format: str, output_dir: str) -> list:
        """Convert multiple files concurrently."""
        tasks = []
        for input_file in files:
            output_file = f"{output_dir}/{Path(input_file).stem}.{to_format}"
            tasks.append(self.convert(input_file, to_format, output_file))
        return await asyncio.gather(*tasks)


# Async usage
async def main():
    cli = AsyncCapsuleCLI("./capsule")

    # Convert multiple files concurrently
    files = ["book1.usfm", "book2.usfm", "book3.usfm"]
    results = await cli.batch_convert(files, "osis", "./output")
    print(f"Converted {len(results)} files")

asyncio.run(main())
```

## Node.js Integration

### Basic Node.js Wrapper

```javascript
const { execSync, spawn } = require('child_process');
const path = require('path');

class CapsuleCLI {
  /**

   * Node.js wrapper for Juniper Bible CLI.
   * @param {string} binaryPath - Path to capsule binary
   */
  constructor(binaryPath = './capsule') {
    this.binary = binaryPath;
  }

  /**

   * Convert file to target format.
   * @param {string} inputPath - Input file path
   * @param {string} toFormat - Target format (osis, epub, etc.)
   * @param {string} outputPath - Output file path
   * @returns {string} Output path
   */
  convert(inputPath, toFormat, outputPath) {
    execSync(`"${this.binary}" convert "${inputPath}" --to ${toFormat} --out "${outputPath}"`);
    return outputPath;
  }

  /**

   * Extract IR from file.
   * @param {string} inputPath - Input file path
   * @param {string} outputPath - Output IR file path
   * @returns {string} Output path
   */
  extractIR(inputPath, outputPath) {
    execSync(`"${this.binary}" extract-ir "${inputPath}" --out "${outputPath}"`);
    return outputPath;
  }

  /**

   * Emit native format from IR.
   * @param {string} irPath - IR file path
   * @param {string} format - Target format
   * @param {string} outputPath - Output file path
   * @returns {string} Output path
   */
  emitNative(irPath, format, outputPath) {
    execSync(`"${this.binary}" emit-native "${irPath}" --format ${format} --out "${outputPath}"`);
    return outputPath;
  }

  /**

   * Detect file format.
   * @param {string} filePath - File to detect
   * @returns {string} Detected format
   */
  detectFormat(filePath) {
    return execSync(`"${this.binary}" detect "${filePath}"`).toString().trim();
  }

  /**

   * List available plugins.
   * @returns {string} Plugin listing
   */
  listPlugins() {
    return execSync(`"${this.binary}" plugins`).toString();
  }

  /**

   * Ingest file into capsule.
   * @param {string} inputPath - Input file or directory
   * @param {string} outputPath - Output capsule path
   * @returns {string} Output path
   */
  ingest(inputPath, outputPath) {
    execSync(`"${this.binary}" ingest "${inputPath}" --out "${outputPath}"`);
    return outputPath;
  }

  /**

   * Verify capsule integrity.
   * @param {string} capsulePath - Capsule file path
   * @returns {boolean} True if valid
   */
  verify(capsulePath) {
    try {
      execSync(`"${this.binary}" verify "${capsulePath}"`);
      return true;
    } catch {
      return false;
    }
  }

  /**

   * Convert file asynchronously (streaming).
   * @param {string} inputPath - Input file path
   * @param {string} toFormat - Target format
   * @param {string} outputPath - Output file path
   * @returns {Promise<string>} Output path
   */
  convertAsync(inputPath, toFormat, outputPath) {
    return new Promise((resolve, reject) => {
      const proc = spawn(this.binary, ['convert', inputPath, '--to', toFormat, '--out', outputPath]);

      let stderr = '';
      proc.stderr.on('data', (data) => { stderr += data; });

      proc.on('close', (code) => {
        if (code === 0) {
          resolve(outputPath);
        } else {
          reject(new Error(`Conversion failed: ${stderr}`));
        }
      });
    });
  }

  /**

   * Batch convert files concurrently.
   * @param {string[]} files - Input files
   * @param {string} toFormat - Target format
   * @param {string} outputDir - Output directory
   * @returns {Promise<string[]>} Output paths
   */
  async batchConvert(files, toFormat, outputDir) {
    const promises = files.map(file => {
      const basename = path.basename(file, path.extname(file));
      const outputPath = path.join(outputDir, `${basename}.${toFormat}`);
      return this.convertAsync(file, toFormat, outputPath);
    });
    return Promise.all(promises);
  }
}

// Usage
const cli = new CapsuleCLI('./capsule');

// Sync usage
cli.convert('input.usfm', 'osis', 'output.osis');

// Async usage
(async () => {
  const files = ['book1.usfm', 'book2.usfm'];
  const results = await cli.batchConvert(files, 'osis', './output');
  console.log(`Converted ${results.length} files`);
})();

module.exports = CapsuleCLI;
```

### TypeScript Definitions

```typescript
// capsule-cli.d.ts
declare class CapsuleCLI {
  constructor(binaryPath?: string);
  convert(inputPath: string, toFormat: string, outputPath: string): string;
  extractIR(inputPath: string, outputPath: string): string;
  emitNative(irPath: string, format: string, outputPath: string): string;
  detectFormat(filePath: string): string;
  listPlugins(): string;
  ingest(inputPath: string, outputPath: string): string;
  verify(capsulePath: string): boolean;
  convertAsync(inputPath: string, toFormat: string, outputPath: string): Promise<string>;
  batchConvert(files: string[], toFormat: string, outputDir: string): Promise<string[]>;
}

export = CapsuleCLI;
```

## Shell Script Library

### capsule-lib.sh

```bash
#!/bin/bash
# capsule-lib.sh - Shell library for Juniper Bible
# Source this file to use the functions: source capsule-lib.sh

CAPSULE_BIN="${CAPSULE_BIN:-./capsule}"

# Convert file to target format
# Usage: capsule_convert input.usfm osis output.osis
capsule_convert() {
    local input="$1"
    local format="$2"
    local output="$3"
    "$CAPSULE_BIN" convert "$input" --to "$format" --out "$output"
}

# Batch convert all files with extension to format
# Usage: capsule_batch_convert ./input/ usfm osis ./output/
capsule_batch_convert() {
    local input_dir="$1"
    local from_ext="$2"
    local to_format="$3"
    local output_dir="$4"

    mkdir -p "$output_dir"
    for f in "$input_dir"/*."$from_ext"; do
        if [ -f "$f" ]; then
            base=$(basename "$f" ".$from_ext")
            echo "Converting: $f -> $output_dir/$base.$to_format"
            capsule_convert "$f" "$to_format" "$output_dir/$base.$to_format"
        fi
    done
}

# Detect file format
# Usage: format=$(capsule_detect myfile.xml)
capsule_detect() {
    "$CAPSULE_BIN" detect "$1"
}

# Verify capsule integrity
# Usage: if capsule_verify my.capsule.tar.xz; then echo "Valid"; fi
capsule_verify() {
    "$CAPSULE_BIN" verify "$1" >/dev/null 2>&1
}

# Ingest file into capsule
# Usage: capsule_ingest ./mydata/ output.capsule.tar.xz
capsule_ingest() {
    local input="$1"
    local output="$2"
    "$CAPSULE_BIN" ingest "$input" --out "$output"
}

# Extract IR from file
# Usage: capsule_extract_ir input.osis output.ir.json
capsule_extract_ir() {
    local input="$1"
    local output="$2"
    "$CAPSULE_BIN" extract-ir "$input" --out "$output"
}

# Emit native format from IR
# Usage: capsule_emit_native input.ir.json osis output.osis
capsule_emit_native() {
    local input="$1"
    local format="$2"
    local output="$3"
    "$CAPSULE_BIN" emit-native "$input" --format "$format" --out "$output"
}

# List plugins
# Usage: capsule_plugins
capsule_plugins() {
    "$CAPSULE_BIN" plugins
}

# Get IR info
# Usage: capsule_ir_info input.ir.json
capsule_ir_info() {
    "$CAPSULE_BIN" ir-info "$1"
}
```

### Usage Example

```bash
#!/bin/bash
source capsule-lib.sh

# Set custom binary path (optional)
export CAPSULE_BIN="/usr/local/bin/capsule"

# Convert single file
capsule_convert book.usfm osis book.osis

# Batch convert directory
capsule_batch_convert ./usfm-files/ usfm osis ./osis-output/

# Detect and convert
format=$(capsule_detect unknown-file.xml)
echo "Detected format: $format"

# Verify capsule
if capsule_verify my.capsule.tar.xz; then
    echo "Capsule is valid"
else
    echo "Capsule is corrupted!"
fi
```

## Go Integration

### Direct Go Library Usage

For Go applications, you can import the packages directly:

```go
package main

import (
    "log"

    "capsule/core/capsule"
    "capsule/core/ir"
    "capsule/core/plugins"
)

func main() {
    // Load plugins
    loader := plugins.NewLoader()
    loader.LoadFromDir("plugins/")

    // Get format plugin
    plugin, err := loader.GetPlugin("format.osis")
    if err != nil {
        log.Fatal(err)
    }

    // Execute extract-ir
    req := plugins.NewExtractIRRequest("/path/to/bible.osis", "/tmp/output")
    resp, err := plugins.ExecutePlugin(plugin, req)
    if err != nil {
        log.Fatal(err)
    }

    result, err := plugins.ParseExtractIRResult(resp)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("IR extracted to: %s (loss class: %s)", result.IRPath, result.LossClass)
}
```

### Go CLI Wrapper

```go
package main

import (
    "encoding/json"
    "net/http"
    "os/exec"
)

// CapsuleCLI wraps the capsule command line tool
type CapsuleCLI struct {
    Binary string
}

// NewCapsuleCLI creates a new CLI wrapper
func NewCapsuleCLI(binary string) *CapsuleCLI {
    if binary == "" {
        binary = "./capsule"
    }
    return &CapsuleCLI{Binary: binary}
}

// Convert converts a file to target format
func (c *CapsuleCLI) Convert(input, toFormat, output string) error {
    cmd := exec.Command(c.Binary, "convert", input, "--to", toFormat, "--out", output)
    return cmd.Run()
}

// DetectFormat detects the format of a file
func (c *CapsuleCLI) DetectFormat(path string) (string, error) {
    cmd := exec.Command(c.Binary, "detect", path)
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return string(out), nil
}

// REST API example
func main() {
    cli := NewCapsuleCLI("./capsule")

    http.HandleFunc("/api/convert", func(w http.ResponseWriter, r *http.Request) {
        input := r.FormValue("input")
        format := r.FormValue("format")
        output := r.FormValue("output")

        err := cli.Convert(input, format, output)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        json.NewEncoder(w).Encode(map[string]string{
            "status": "success",
            "output": output,
        })
    })

    http.ListenAndServe(":8080", nil)
}
```

## REST API Wrapper Pattern

A common pattern is to wrap the CLI behind a REST API:

```
POST /api/convert
  Body: {input: string, format: string, output: string}

GET /api/detect?path=file.xml

POST /api/ingest
  Body: {input: string, output: string}

GET /api/plugins

POST /api/extract-ir
  Body: {input: string, output: string}
```

See the Go example above for a basic implementation.

## Error Handling

The CLI uses standard exit codes:

- `0`: Success
- `1`: General error
- `2`: Invalid arguments

Always check the return code and stderr for error messages.

## Performance Tips

1. **Batch Operations**: Use batch conversion for multiple files
2. **Parallel Processing**: CLI operations are independent - run them in parallel
3. **Direct Library**: For Go apps, import packages directly instead of CLI wrapper
4. **Caching**: IR files can be cached and reused for multiple output formats

## Related Documentation

- [QUICK_START.md](QUICK_START.md) - Basic usage guide
- [CLI_REFERENCE.md](generated/CLI_REFERENCE.md) - Complete CLI reference
- [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) - Creating custom plugins
