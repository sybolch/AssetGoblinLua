package assetgoblinlua

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/regen-network/gocuke"
)

func TestLuaFeatures(t *testing.T) {
	gocuke.NewRunner(t, &luaSuite{}).Path("features.feature").
		Step(`^Lua source (.*)$`, (*luaSuite).Source).
		Step(`^Lua initialization exceeding "([^"]*)"$`, (*luaSuite).Limit).
		Step(`^the Lua source is loaded$`, (*luaSuite).Load).
		Step(`^loading succeeds$`, (*luaSuite).Success).
		Step(`^loading fails$`, (*luaSuite).Failure).
		Step(`^calling the callback returns "([^"]*)"$`, (*luaSuite).Call).
		Step(`^the base is recorded as an ancestor$`, (*luaSuite).Ancestor).
		Step(`^the parent source changes$`, (*luaSuite).ChangeParent).
		Step(`^the revision changes$`, (*luaSuite).Revision).
		Step(`^a cancelled callback fails$`, (*luaSuite).Cancel).
		Step(`^empty declarations and codec callbacks are available$`, (*luaSuite).Codec).
		Run()
}

type luaSuite struct {
	gocuke.TestingT
	directory          string
	value              any
	parents            []string
	revision, previous string
	close              func()
	err                error
}

func (s *luaSuite) Before() {
	s.close = nil
	s.value = nil
	s.parents = nil
	s.revision, s.previous = "", ""
	s.directory, s.err = os.MkdirTemp("", "assetgoblin-lua-")
	if s.err != nil {
		s.Fatal(s.err)
	}
	s.Source(`return {api_version=1,routes={}}`)
	s.write("base.lua", `return {api_version=1,routes={{path='/files/',handle=function(r)return {body='base'} end}}}`)
}
func (s *luaSuite) After() {
	if s.close != nil {
		s.close()
		s.close = nil
	}
	os.RemoveAll(s.directory)
}
func (s *luaSuite) write(name, source string) {
	s.Helper()
	if err := os.WriteFile(filepath.Join(s.directory, name), []byte(source), 0600); err != nil {
		s.Fatal(err)
	}
}
func (s *luaSuite) Source(source string) { s.write("custom.lua", source) }
func (s *luaSuite) Limit(limit string) {
	if limit == "size" {
		s.Source(strings.Repeat(" ", maxScriptSize+1))
	} else {
		s.Source("while true do end")
	}
}
func (s *luaSuite) Load() {
	if s.close != nil {
		s.close()
		s.close = nil
	}
	s.previous = s.revision
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	s.value, s.parents, s.revision, s.close, s.err = (Backend{}).Load(ctx, filepath.Join(s.directory, "custom.lua"), map[string]string{"custom": filepath.Join(s.directory, "custom.lua"), "base": filepath.Join(s.directory, "base.lua"), "python": filepath.Join(s.directory, "python.py")})
}
func (s *luaSuite) Success() {
	s.Helper()
	if s.err != nil {
		s.Fatal(s.err)
	}
	if s.close == nil {
		s.Fatal("missing closer")
	}
}
func (s *luaSuite) Failure() {
	s.Helper()
	if s.err == nil {
		s.Fatal("invalid source accepted")
	}
}
func (s *luaSuite) callback() func(context.Context, map[string]any) (any, error) {
	s.Helper()
	return s.value.(map[string]any)["routes"].([]any)[0].(map[string]any)["handle"].(func(context.Context, map[string]any) (any, error))
}
func (s *luaSuite) Call(expected string) {
	s.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	value, err := s.callback()(ctx, map[string]any{"path": "hello", "method": "GET", "query": map[string]any{"name": "world"}, "headers": map[string]any{"x-test": "value"}})
	if err != nil {
		s.Fatal(err)
	}
	if value.(map[string]any)["body"] != expected {
		s.Fatalf("unexpected callback result: %v", value)
	}
}
func (s *luaSuite) Ancestor() {
	s.Helper()
	if len(s.parents) != 1 || s.parents[0] != filepath.Join(s.directory, "base.lua") {
		s.Fatalf("unexpected ancestors: %v", s.parents)
	}
}
func (s *luaSuite) ChangeParent() {
	s.write("base.lua", `return {api_version=1,routes={{path='/files/',handle=function(r)return {body='updated'} end}}}`)
}
func (s *luaSuite) Revision() {
	s.Helper()
	if s.revision == s.previous || s.revision == "" {
		s.Fatal("unchanged revision")
	}
}
func (s *luaSuite) Cancel() {
	s.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := s.callback()(ctx, map[string]any{"path": "loop"}); err == nil {
		s.Fatal("cancellation ignored")
	}
}
func (s *luaSuite) Codec() {
	s.Helper()
	v := s.value.(map[string]any)
	for _, key := range []string{"tools", "routes"} {
		if a, ok := v[key].([]any); !ok || len(a) != 0 {
			s.Fatalf("empty array lost: %s", key)
		}
	}
	codec := v["image_codecs"].(map[string]any)["custom"].(map[string]any)
	for _, key := range []string{"encode", "decode"} {
		if _, ok := codec[key].(func(context.Context, map[string]any) (any, error)); !ok {
			s.Fatalf("missing callback %s", key)
		}
	}
}
