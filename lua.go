// Package assetgoblinlua provides the optional Lua runtime for AssetGoblin.
package assetgoblinlua

import (
	"context"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// Backend implements AssetGoblin's plugin.Backend interface without depending on the host.
// Register Backend{} with plugin.RegisterBackend before loading plugins.
type Backend struct{}

func (Backend) Extension() string { return ".lua" }

// Load returns a manifest, its ancestors, a content revision and an interpreter closer.
// The caller serializes callbacks and Close, and supplies initialization/callback deadlines.
func (Backend) Load(ctx context.Context, filename string, catalog map[string]string) (_ any, _ []string, _ string, _ func(), err error) {
	state := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer func() {
		if err != nil {
			state.Close()
		}
	}()
	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase}, {lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString}, {lua.MathLibName, lua.OpenMath},
	} {
		if err = state.CallByParam(lua.P{Fn: state.NewFunction(lib.open), NRet: 0, Protect: true}, lua.LString(lib.name)); err != nil {
			return nil, nil, "", nil, err
		}
	}
	// No host filesystem, process, package loader, or debug access from Lua.
	for _, name := range []string{"dofile", "loadfile", "print"} {
		state.SetGlobal(name, lua.LNil)
	}
	state.SetContext(ctx)
	defer state.RemoveContext()
	manifest, parents, revision, err := evaluateManifest(state, filename, catalog, map[string]bool{})
	if err != nil {
		return nil, nil, "", nil, err
	}
	value, err := fromLua(state, manifest)
	if err != nil {
		return nil, nil, "", nil, err
	}
	return value, parents, revision, state.Close, nil
}

func fromLua(state *lua.LState, value lua.LValue) (any, error) { return fromLuaDepth(state, value, 0) }

func fromLuaDepth(state *lua.LState, value lua.LValue, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("Lua table nesting exceeds 64 levels")
	}
	switch value := value.(type) {
	case *lua.LFunction:
		return func(ctx context.Context, request map[string]any) (any, error) {
			state.SetContext(ctx)
			defer state.RemoveContext()
			top := state.GetTop()
			defer state.SetTop(top)
			if err := state.CallByParam(lua.P{Fn: value, NRet: 1, Protect: true}, toLua(state, request)); err != nil {
				return nil, err
			}
			return fromLua(state, state.Get(-1))
		}, nil
	case *lua.LTable:
		if value.Len() > 0 {
			result := make([]any, value.Len())
			for i := range result {
				v, err := fromLuaDepth(state, value.RawGetInt(i+1), depth+1)
				if err != nil {
					return nil, err
				}
				result[i] = v
			}
			return result, nil
		}
		result := map[string]any{}
		var convertErr error
		value.ForEach(func(k, v lua.LValue) {
			name, ok := k.(lua.LString)
			if !ok {
				convertErr = fmt.Errorf("table keys must be strings")
				return
			}
			converted, err := fromLuaDepth(state, v, depth+1)
			if err != nil {
				convertErr = err
				return
			}
			if table, ok := converted.(map[string]any); ok && len(table) == 0 {
				switch string(name) {
				case "routes", "tools", "args", "processes":
					converted = []any{}
				}
			}
			result[string(name)] = converted
		})
		return result, convertErr
	case lua.LString:
		return string(value), nil
	case lua.LNumber:
		return float64(value), nil
	case lua.LBool:
		return bool(value), nil
	case *lua.LNilType:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported Lua value")
	}
}

func toLua(state *lua.LState, value any) lua.LValue {
	switch value := value.(type) {
	case string:
		return lua.LString(value)
	case map[string]any:
		table := state.NewTable()
		for k, v := range value {
			table.RawSetString(k, toLua(state, v))
		}
		return table
	default:
		return lua.LNil
	}
}
