package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

const (
	PluginName  = "icloud"
	Service     = "li.lds.age-plugin-icloud"
	BundleID    = "li.lds.age-plugin-icloud"
	DefaultName = "default"
)

// AccessControlMode is an annotation on a stored identity. iCloud Keychain
// cannot attach SecAccessControl to synchronizable items, so this is not
// enforced by Keychain. Decrypt may call LAContext.EvaluatePolicy.
type AccessControlMode string

const (
	AccessControlNone      AccessControlMode = "none"
	AccessControlEveryTime AccessControlMode = "everyTime"
	AccessControl5m        AccessControlMode = "5m"
	presenceCache                            = 5 * time.Minute
)

func ParseAccessControl(s string) (AccessControlMode, error) {
	switch s {
	case string(AccessControlNone):
		return AccessControlNone, nil
	case string(AccessControlEveryTime), "always":
		return AccessControlEveryTime, nil
	case string(AccessControl5m), "cache5m", "":
		return AccessControl5m, nil
	case "userPresence":
		// Stored by earlier builds; treat as 5m cache.
		return AccessControl5m, nil
	default:
		return "", fmt.Errorf("unknown --access-control %q (want none, everyTime, or 5m)", s)
	}
}

func (m AccessControlMode) prompts() bool {
	return m == AccessControlEveryTime || m == AccessControl5m
}

// publicAttrs is stored in kSecAttrGeneric (attributes-only; no secret).
// AccessControl is an annotation for an app-level LAContext prompt on decrypt.
// iCloud Keychain rejects kSecAttrAccessControl on synchronizable items.
// PresenceAt is last successful prompt per Mac (IOPlatformUUID → unix time),
// used only by the 5m policy.
type publicAttrs struct {
	Recipient     string           `json:"recipient"`
	AccessControl string           `json:"accessControl,omitempty"`
	PresenceAt    map[string]int64 `json:"presenceAt,omitempty"`
}

func (a publicAttrs) mode() AccessControlMode {
	if a.AccessControl == "" {
		return AccessControlNone
	}
	m, err := ParseAccessControl(a.AccessControl)
	if err != nil {
		return AccessControlNone
	}
	return m
}

func (a publicAttrs) presenceFresh(machine string, now time.Time) bool {
	if a.mode() != AccessControl5m || machine == "" || a.PresenceAt == nil {
		return false
	}
	at := a.PresenceAt[machine]
	if at == 0 {
		return false
	}
	d := now.Sub(time.Unix(at, 0))
	return d >= 0 && d < presenceCache
}

func (a *publicAttrs) stampPresence(machine string, now time.Time) {
	if machine == "" {
		return
	}
	if a.PresenceAt == nil {
		a.PresenceAt = make(map[string]int64)
	}
	a.PresenceAt[machine] = now.Unix()
}

func encodePublicAttrs(recipient string, mode AccessControlMode) ([]byte, error) {
	if mode == "" {
		mode = AccessControl5m
	}
	return json.Marshal(publicAttrs{
		Recipient:     recipient,
		AccessControl: string(mode),
	})
}

func parsePublicAttrs(raw []byte) (publicAttrs, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return publicAttrs{}, nil
	}
	if s[0] != '{' {
		return publicAttrs{Recipient: s}, nil
	}
	var a publicAttrs
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return publicAttrs{}, fmt.Errorf("item attributes: %w", err)
	}
	if a.AccessControl != "" {
		if _, err := ParseAccessControl(a.AccessControl); err != nil {
			return publicAttrs{}, err
		}
	}
	return a, nil
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func ValidateName(name string) error {
	if name == "" {
		return errors.New("identity name is required")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid identity name %q (use letters, digits, '.', '_' or '-', max 64)", name)
	}
	return nil
}

// EncodeIdentity returns the non-secret AGE-PLUGIN-ICLOUD pointer for name.
func EncodeIdentity(name string) string {
	return plugin.EncodeIdentity(PluginName, []byte(name))
}

// IdentityFile is the on-disk pointer file printed by --generate and --list.
func IdentityFile(name, recipient string, created time.Time, mode AccessControlMode) string {
	var b strings.Builder
	if !created.IsZero() {
		fmt.Fprintf(&b, "# created: %s\n", created.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "# name: %s\n", name)
	fmt.Fprintf(&b, "# recipient: %s\n", recipient)
	fmt.Fprintf(&b, "# access-control: %s\n", mode)
	fmt.Fprintf(&b, "%s\n", EncodeIdentity(name))
	return b.String()
}

// Item is a stored identity's public attributes.
type Item struct {
	Name          string
	Recipient     string
	AccessControl AccessControlMode
}

type multiIdentity []age.Identity

func (m multiIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	for _, id := range m {
		fileKey, err := id.Unwrap(stanzas)
		if err == nil {
			return fileKey, nil
		}
		if !errors.Is(err, age.ErrIncorrectIdentity) {
			return nil, err
		}
	}
	return nil, age.ErrIncorrectIdentity
}
