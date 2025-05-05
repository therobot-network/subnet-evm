package llmprecompile

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// Holds the ABI strings for each primitive
var primitiveABI = map[string]string{}

func init() {
	loadABI("counter", "primitiveABIs/CounterPrimitive.json")
	loadABI("erc20", "primitiveABIs/ERC20Primitive.json")
	loadABI("math", "primitiveABIs/MathPrimitive.json")
	loadABI("amm", "primitiveABIs/AmmPrimitive.json")
	loadABI("PythonPrimitive", "primitiveABIs/PythonPrimitive.json")
	loadABI("SystemPrimitive", "primitiveABIs/SystemPrimitive.json")
	// Add more as needed...
}

func loadABI(key, relativePath string) {
	// Get the directory of the current file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("Unable to get current filename for ABI resolution")
	}

	// Resolve path relative to current file
	baseDir := filepath.Dir(filename)
	absPath := filepath.Join(baseDir, relativePath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", absPath, err)
	}

	var parsed struct {
		ABI json.RawMessage `json:"abi"`
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		log.Fatalf("Failed to parse ABI JSON for %s: %v", key, err)
	}

	primitiveABI[key] = string(parsed.ABI)
}
