package identity

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

func TestValidateName(t *testing.T) {
	ok := []string{"default", "work", "a", "A1._-b"}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q): %v", n, err)
		}
	}
	bad := []string{"", " has space", "no/slash", strings.Repeat("a", 65)}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q): want error", n)
		}
	}
}

func TestEncodeIdentity(t *testing.T) {
	s := EncodeIdentity("default")
	name, data, err := plugin.ParseIdentity(s)
	if err != nil {
		t.Fatal(err)
	}
	if name != PluginName {
		t.Fatalf("plugin name: got %q", name)
	}
	if string(data) != "default" {
		t.Fatalf("payload: got %q", data)
	}

	empty := EncodeIdentity("")
	name, data, err = plugin.ParseIdentity(empty)
	if err != nil {
		t.Fatal(err)
	}
	if name != PluginName || len(data) != 0 {
		t.Fatalf("empty identity: name=%q data=%q", name, data)
	}
}

func TestIdentityFile(t *testing.T) {
	created := time.Date(2026, 9, 4, 12, 37, 0, 0, time.FixedZone("CEST", 2*3600))
	got := IdentityFile("default", "age1pq1example", created, AccessControlUserPresence)
	wantPrefix := "# created: 2026-09-04T12:37:00+02:00\n# recipient: age1pq1example\n# access-control: userPresence\nAGE-PLUGIN-ICLOUD-"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("got:\n%s", got)
	}
	if strings.Contains(got, "AGE-SECRET-KEY") {
		t.Fatal("identity file must not contain the secret key")
	}

	reprint := IdentityFile("default", "age1pq1example", time.Time{}, AccessControlNone)
	if strings.Contains(reprint, "created:") {
		t.Fatal("reprint must not invent a created timestamp")
	}
	if !strings.HasPrefix(reprint, "# recipient: age1pq1example\n# access-control: none\nAGE-PLUGIN-ICLOUD-") {
		t.Fatalf("reprint:\n%s", reprint)
	}
}

func TestParseAccessControl(t *testing.T) {
	m, err := ParseAccessControl("userPresence")
	if err != nil || m != AccessControlUserPresence {
		t.Fatalf("got %q %v", m, err)
	}
	m, err = ParseAccessControl("")
	if err != nil || m != AccessControlUserPresence {
		t.Fatalf("empty: got %q %v", m, err)
	}
	m, err = ParseAccessControl("none")
	if err != nil || m != AccessControlNone {
		t.Fatalf("got %q %v", m, err)
	}
	if _, err := ParseAccessControl("biometry"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPublicAttrs(t *testing.T) {
	raw, err := encodePublicAttrs("age1pq1example", AccessControlUserPresence)
	if err != nil {
		t.Fatal(err)
	}
	rec, mode, err := parsePublicAttrs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if rec != "age1pq1example" || mode != AccessControlUserPresence {
		t.Fatalf("got rec=%q mode=%q", rec, mode)
	}

	rec, mode, err = parsePublicAttrs([]byte("age1pq1legacy"))
	if err != nil {
		t.Fatal(err)
	}
	if rec != "age1pq1legacy" || mode != AccessControlNone {
		t.Fatalf("legacy: rec=%q mode=%q", rec, mode)
	}

	rec, mode, err = parsePublicAttrs([]byte(`{"recipient":"age1pq1x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rec != "age1pq1x" || mode != AccessControlNone {
		t.Fatalf("no annotation: rec=%q mode=%q", rec, mode)
	}
}

func TestMultiIdentity(t *testing.T) {
	a, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatal(err)
	}
	b, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatal(err)
	}

	fileKey := make([]byte, 16)
	if _, err := rand.Read(fileKey); err != nil {
		t.Fatal(err)
	}
	stanzas, err := a.Recipient().Wrap(fileKey)
	if err != nil {
		t.Fatal(err)
	}

	got, err := multiIdentity{b, a}.Unwrap(stanzas)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fileKey) {
		t.Fatal("unwrapped file key mismatch")
	}

	if _, err := (multiIdentity{b}).Unwrap(stanzas); err == nil {
		t.Fatal("expected incorrect identity")
	}
}
