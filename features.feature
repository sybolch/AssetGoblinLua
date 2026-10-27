Feature: Optional Lua backend
  Scenario: Lua callbacks retain state and receive request metadata
    Given Lua source return {api_version=1,routes={{path='/files/',handle=function(r) count=(count or 0)+1;return {body=r.path..':'..count} end}}}
    When the Lua source is loaded
    Then loading succeeds
    And calling the callback returns "hello:1"
    And calling the callback returns "hello:2"

  Scenario Outline: Invalid or restricted Lua scripts fail initialization
    Given Lua source <source>
    When the Lua source is loaded
    Then loading fails
    Examples:
      | source |
      | return { |
      | return false |
      | return {api_version=2} |
      | os.execute('echo forbidden') |
      | dofile('anything') |
      | require('io') |
      | debug.traceback() |
      | return {api_version=1,extends='missing',extend=function(p)return p end} |
      | return {api_version=1,extends='custom',extend=function(p)return p end} |
      | return {api_version=1,extends='base'} |
      | return {api_version=1,extends='base',extend=function(p)return false end} |
      | return {api_version=1,extends='python',extend=function(p)return p end} |
      | local t={};t.loop=t;return {api_version=1,routes=t} |

  Scenario Outline: Initialization respects source and time limits
    Given Lua initialization exceeding "<limit>"
    When the Lua source is loaded
    Then loading fails
    Examples:
      | limit |
      | size |
      | time |

  Scenario: Parent callbacks compose inside the leaf interpreter
    Given Lua source return {api_version=1,extends='base',extend=function(p)local old=p.routes[1].handle;p.routes[1].handle=function(r)local v=old(r);v.body=v.body..':child';return v end;return p end}
    When the Lua source is loaded
    Then loading succeeds
    And calling the callback returns "base:child"
    And the base is recorded as an ancestor
    When the parent source changes
    And the Lua source is loaded
    Then the revision changes

  Scenario: Cancellation interrupts callbacks without destroying the interpreter
    Given Lua source return {api_version=1,routes={{path='/files/',handle=function(r)if r.path=='loop' then while true do end end;return {body=r.path} end}}}
    When the Lua source is loaded
    Then loading succeeds
    And a cancelled callback fails
    And calling the callback returns "hello"

  Scenario: Manifest conversion supports empty arrays and codec callbacks
    Given Lua source return {api_version=1,tools={},routes={},image_codecs={custom={mime_type='image/custom',decode=function(r)return {args={r.path,'out'}} end,encode=function(r)return {} end}}}
    When the Lua source is loaded
    Then loading succeeds
    And empty declarations and codec callbacks are available

  Scenario: Request metadata crosses the runtime boundary
    Given Lua source return {api_version=1,routes={{path='/files/',handle=function(r)return {body=r.method..':'..r.query.name..':'..r.headers['x-test']} end}}}
    When the Lua source is loaded
    Then loading succeeds
    And calling the callback returns "GET:world:value"

  Scenario: The interpreter remains available after callback exceptions
    Given Lua source return {api_version=1,routes={{path='/files/',handle=function(r)if r.path=='loop' then error('deliberate error') end;return {body=r.path} end}}}
    When the Lua source is loaded
    Then loading succeeds
    And a cancelled callback fails
    And calling the callback returns "hello"
