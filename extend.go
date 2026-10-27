package assetgoblinlua

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const maxScriptSize = 1 << 20

// evaluateManifest composes dependencies inside the leaf plugin's own interpreter.
// Parent closures remain valid; unrelated plugins retain isolated Lua states.
func evaluateManifest(state *lua.LState, filename string, catalog map[string]string, stack map[string]bool) (*lua.LTable, []string, string, error) {
	if stack[filename] || len(stack) >= 16 {
		return nil, nil, "", fmt.Errorf("cyclic or excessively deep plugin inheritance at %s", filename)
	}
	stack[filename] = true
	defer delete(stack, filename)
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, "", err
	}
	source, err := io.ReadAll(io.LimitReader(file, maxScriptSize+1))
	closeErr := file.Close()
	if err != nil {
		return nil, nil, "", err
	}
	if closeErr != nil {
		return nil, nil, "", closeErr
	}
	if len(source) > maxScriptSize {
		return nil, nil, "", fmt.Errorf("script exceeds %d bytes", maxScriptSize)
	}
	fn, err := state.Load(strings.NewReader(string(source)), filename)
	if err != nil {
		return nil, nil, "", err
	}
	// Global assignments are local to this script; standard libraries remain available.
	environment := state.NewTable()
	meta := state.NewTable()
	meta.RawSetString("__index", state.G.Global)
	state.SetMetatable(environment, meta)
	fn.Env = environment
	if err := state.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return nil, nil, "", err
	}
	manifest, ok := state.Get(-1).(*lua.LTable)
	state.Pop(1)
	if !ok {
		return nil, nil, "", fmt.Errorf("script must return a plugin table")
	}
	if manifest.RawGetString("api_version") != lua.LNumber(1) {
		return nil, nil, "", fmt.Errorf("unsupported api_version (expected 1)")
	}
	revision := fmt.Sprintf("%x", sha256.Sum256(source))
	parents := []string{}
	if value := manifest.RawGetString("extends"); value != lua.LNil {
		name, ok := value.(lua.LString)
		if !ok {
			return nil, nil, "", fmt.Errorf("extends must name a plugin")
		}
		parentFile, exists := catalog[string(name)]
		if !exists {
			return nil, nil, "", fmt.Errorf("missing parent plugin %q", name)
		}
		if filepath.Ext(parentFile) != ".lua" {
			return nil, nil, "", fmt.Errorf("cross-language inheritance is not supported")
		}
		parent, ancestors, parentRevision, err := evaluateManifest(state, parentFile, catalog, stack)
		if err != nil {
			return nil, nil, "", err
		}
		extend, ok := manifest.RawGetString("extend").(*lua.LFunction)
		if !ok {
			return nil, nil, "", fmt.Errorf("extends requires extend(parent) function")
		}
		if err := state.CallByParam(lua.P{Fn: extend, NRet: 1, Protect: true}, parent); err != nil {
			return nil, nil, "", err
		}
		manifest, ok = state.Get(-1).(*lua.LTable)
		state.Pop(1)
		if !ok {
			return nil, nil, "", fmt.Errorf("extend(parent) must return a plugin table")
		}
		parents = append(ancestors, parentFile)
		revision = fmt.Sprintf("%x", sha256.Sum256([]byte(revision+parentRevision)))
	}
	return manifest, parents, revision, nil
}
