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
// enforced by Keychain; decrypt will honor it later with an app-level prompt.
type AccessControlMode string

const (
	AccessControlUserPresence AccessControlMode = "userPresence"
	AccessControlNone         AccessControlMode = "none"
)

func ParseAccessControl(s string) (AccessControlMode, error) {
	switch AccessControlMode(s) {
	case AccessControlUserPresence, "":
		return AccessControlUserPresence, nil
	case AccessControlNone:
		return AccessControlNone, nil
	default:
		return "", fmt.Errorf("unknown --access-control %q (want userPresence or none)", s)
	}
}

// publicAttrs is stored in kSecAttrGeneric (attributes-only; no secret).
// AccessControl is an annotation for a later app-level LAContext prompt.
// iCloud Keychain rejects kSecAttrAccessControl on synchronizable items.
type publicAttrs struct {
	Recipient     string `json:"recipient"`
	AccessControl string `json:"accessControl,omitempty"`
}

func encodePublicAttrs(recipient string, mode AccessControlMode) ([]byte, error) {
	if mode == "" {
		mode = AccessControlUserPresence
	}
	return json.Marshal(publicAttrs{
		Recipient:     recipient,
		AccessControl: string(mode),
	})
}

func parsePublicAttrs(raw []byte) (recipient string, mode AccessControlMode, err error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "", AccessControlNone, nil
	}
	if s[0] != '{' {
		return s, AccessControlNone, nil
	}
	var a publicAttrs
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return "", "", fmt.Errorf("item attributes: %w", err)
	}
	mode, err = ParseAccessControl(a.AccessControl)
	if err != nil {
		return "", "", err
	}
	if a.AccessControl == "" {
		mode = AccessControlNone
	}
	return a.Recipient, mode, nil
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
