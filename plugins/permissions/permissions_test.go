package permissions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.minekube.com/gate/pkg/util/permission"
	"go.minekube.com/gate/pkg/util/uuid"
)

const testUUID = "069a79f4-44e9-4726-a5be-fca90e38aaf5"

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("uuid.Parse(%q): %v", s, err)
	}
	return id
}

func mustParse(t *testing.T, yml string) *Provider {
	t.Helper()
	p, err := Parse([]byte(yml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestDefaultGrantsApplyToUnlistedPlayer(t *testing.T) {
	p := mustParse(t, `
default:
  - gate.command.server
`)
	fn := p.Func(uuid.New(), "Stranger")
	if got := fn("gate.command.server"); got != permission.True {
		t.Fatalf("/server = %v, want True", got)
	}
	if got := fn("gate.command.glist"); got != permission.Undefined {
		t.Fatalf("/glist = %v, want Undefined (not granted)", got)
	}
	if fn("gate.command.glist").Bool() {
		t.Fatal("Undefined must evaluate to false")
	}
}

func TestPlayerEntryReplacesDefault(t *testing.T) {
	p := mustParse(t, `
default:
  - gate.command.server
players:
  "`+testUUID+`":
    - gate.command.glist
`)
	id := mustUUID(t, testUUID)
	fn := p.Func(id, "Notch")
	if got := fn("gate.command.glist"); got != permission.True {
		t.Fatalf("/glist = %v, want True", got)
	}
	if got := fn("gate.command.server"); got != permission.Undefined {
		t.Fatalf("/server = %v, want Undefined: a players entry replaces default", got)
	}
	// The same UUID under another name still matches.
	if got := p.Func(id, "RenamedPlayer")("gate.command.glist"); got != permission.True {
		t.Fatal("UUID key must match regardless of username")
	}
}

func TestUsernameKeyIsCaseInsensitive(t *testing.T) {
	p := mustParse(t, `
players:
  "Notch":
    - gate.command.*
`)
	fn := p.Func(uuid.New(), "notch")
	if got := fn("gate.command.send"); got != permission.True {
		t.Fatalf("/send = %v, want True via gate.command.*", got)
	}
	if got := fn("gate.other"); got != permission.Undefined {
		t.Fatalf("gate.other = %v, want Undefined", got)
	}
}

func TestDenyBeatsGrantAndWildcard(t *testing.T) {
	p := mustParse(t, `
players:
  "`+testUUID+`":
    - "*"
deny:
  "`+testUUID+`":
    - gate.command.send
`)
	fn := p.Func(mustUUID(t, testUUID), "Notch")
	if got := fn("gate.command.send"); got != permission.False {
		t.Fatalf("/send = %v, want False", got)
	}
	if got := fn("gate.command.server"); got != permission.True {
		t.Fatalf("/server = %v, want True", got)
	}
}

func TestWildcardMatchesOnlyPrefix(t *testing.T) {
	p := mustParse(t, `
default:
  - gate.command.*
`)
	fn := p.Func(uuid.New(), "Anyone")
	if !fn("gate.command.server").Bool() {
		t.Fatal("prefix wildcard must grant gate.command.server")
	}
	if fn("gate.command").Bool() {
		t.Fatal("prefix wildcard must not grant the bare prefix node")
	}
	if fn("gate.other").Bool() {
		t.Fatal("prefix wildcard must not leak to other nodes")
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	cases := map[string]string{
		"unknown field":   "permisions:\n  - gate.command.server\n",
		"nothing granted": "players: {}\n",
		"empty player key": `
default:
  - gate.command.server
players:
  "  ":
    - gate.command.glist
`,
	}
	for name, yml := range cases {
		if _, err := Parse([]byte(yml)); err == nil {
			t.Fatalf("%s: expected an error, got nil", name)
		}
	}
}

func TestShippedExampleFileIsValid(t *testing.T) {
	p, err := LoadFile(filepath.Join("..", "..", "permissions.yml"))
	if err != nil {
		t.Fatalf("LoadFile(example): %v", err)
	}
	if got := p.Func(uuid.New(), "Anyone")("gate.command.server"); got != permission.True {
		t.Fatalf("example default /server = %v, want True", got)
	}
}

func TestLoadFileMissingIsNotExist(t *testing.T) {
	_, err := LoadFile(filepath.Join(t.TempDir(), "absent.yml"))
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "absent.yml") {
		t.Fatalf("err = %v, want the path in the message", err)
	}
}
