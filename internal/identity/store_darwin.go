//go:build darwin

package identity

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"lds.li/keychain"
)

var (
	mu      sync.Mutex
	secrets = map[string]*age.HybridIdentity{}
	agOnce  sync.Once
	ag      string
	agErr   error
)

func accessGroup() (string, error) {
	agOnce.Do(func() {
		team, err := keychain.TeamIdentifier()
		if err != nil {
			agErr = fmt.Errorf("age-plugin-icloud must be signed with a stable Apple Team ID (see README): %w", err)
			return
		}
		ag = team + "." + BundleID
	})
	return ag, agErr
}

func baseQuery(name string) (keychain.GenericPasswordQuery, error) {
	group, err := accessGroup()
	if err != nil {
		return keychain.GenericPasswordQuery{}, err
	}
	return keychain.GenericPasswordQuery{
		Account:                   name,
		Service:                   Service,
		Synchronizable:            keychain.Bool(true),
		UseDataProtectionKeychain: keychain.Bool(true),
		AccessGroup:               group,
	}, nil
}

func wrapKeychain(op string, err error) error {
	var kcErr *keychain.Error
	if errors.As(err, &kcErr) {
		switch kcErr.Code() {
		case keychain.ErrorCodeMissingEntitlement:
			return fmt.Errorf("%s: iCloud Keychain requires a signed .app with com.apple.application-identifier (see README): %w", op, err)
		case keychain.ErrorCodeItemNotFound:
			return fmt.Errorf("%s: identity not found", op)
		case keychain.ErrorCodeDuplicateItem:
			return fmt.Errorf("%s: an identity with this name already exists", op)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// Generate creates a new MLKEM768-X25519 identity in iCloud Keychain and
// returns the non-secret plugin identity file.
func Generate(name string, mode AccessControlMode) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if mode == "" {
		mode = AccessControlUserPresence
	}
	group, err := accessGroup()
	if err != nil {
		return "", err
	}

	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return "", err
	}
	recipient := id.Recipient().String()
	generic, err := encodePublicAttrs(recipient, mode)
	if err != nil {
		return "", err
	}

	item := keychain.GenericPassword{
		Account:                   name,
		Service:                   Service,
		Label:                     fmt.Sprintf("age-plugin-icloud (%s)", name),
		Value:                     []byte(id.String()),
		GenericAttributes:         generic,
		Synchronizable:            keychain.Bool(true),
		UseDataProtectionKeychain: keychain.Bool(true),
		AccessGroup:               group,
		Accessible:                keychain.AccessibleAfterFirstUnlock,
	}

	if err := keychain.CreateGenericPassword(item); err != nil {
		return "", wrapKeychain("create identity", err)
	}

	return IdentityFile(name, recipient, time.Now(), mode), nil
}

// List returns names and recipients without reading secrets.
func List() ([]Item, error) {
	q, err := baseQuery("")
	if err != nil {
		return nil, err
	}
	passwords, err := keychain.ListGenericPasswords(q)
	if err != nil {
		var kcErr *keychain.Error
		if errors.As(err, &kcErr) && kcErr.Code() == keychain.ErrorCodeItemNotFound {
			return nil, nil
		}
		return nil, wrapKeychain("list identities", err)
	}
	items := make([]Item, 0, len(passwords))
	for _, p := range passwords {
		rec, mode, err := parsePublicAttrs(p.GenericAttributes)
		if err != nil {
			return nil, fmt.Errorf("identity %q: %w", p.Account, err)
		}
		items = append(items, Item{
			Name:          p.Account,
			Recipient:     rec,
			AccessControl: mode,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// Delete removes a named identity from iCloud Keychain (all devices).
func Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	q, err := baseQuery(name)
	if err != nil {
		return err
	}
	if err := keychain.DeleteGenericPassword(q); err != nil {
		return wrapKeychain("delete identity "+name, err)
	}
	mu.Lock()
	delete(secrets, name)
	mu.Unlock()
	return nil
}

// LoadIdentity returns a Keychain-backed HybridIdentity. An empty name loads
// every stored key (for age -j icloud). Secrets are read once per process.
func LoadIdentity(name string) (age.Identity, error) {
	mu.Lock()
	defer mu.Unlock()

	if name == "" {
		items, err := List()
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, errors.New("no identities in iCloud Keychain; run age-plugin-icloud --generate")
		}
		ids := make(multiIdentity, 0, len(items))
		for _, item := range items {
			id, err := loadSecretLocked(item.Name)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, nil
	}

	return loadSecretLocked(name)
}

// LoadRecipient returns the native age1pq recipient from item attributes.
// An empty name uses "default". This does not read the secret.
func LoadRecipient(name string) (age.Recipient, error) {
	if name == "" {
		name = DefaultName
	}
	q, err := baseQuery(name)
	if err != nil {
		return nil, err
	}
	attrs, err := keychain.GetGenericPasswordAttributes(q)
	if err != nil {
		var kcErr *keychain.Error
		if errors.As(err, &kcErr) && kcErr.Code() == keychain.ErrorCodeItemNotFound {
			if name == DefaultName {
				return nil, errors.New("no default identity; run age-plugin-icloud --generate")
			}
			return nil, fmt.Errorf("no identity named %q", name)
		}
		return nil, wrapKeychain("load recipient "+name, err)
	}
	rec, _, err := parsePublicAttrs(attrs.GenericAttributes)
	if err != nil {
		return nil, fmt.Errorf("identity %q: %w", name, err)
	}
	if rec == "" {
		return nil, fmt.Errorf("identity %q has no stored recipient", name)
	}
	return age.ParseHybridRecipient(rec)
}

func loadSecretLocked(name string) (*age.HybridIdentity, error) {
	if id, ok := secrets[name]; ok {
		return id, nil
	}
	q, err := baseQuery(name)
	if err != nil {
		return nil, err
	}
	secret, err := keychain.GetGenericPassword(q)
	if err != nil {
		return nil, wrapKeychain("load identity "+name, err)
	}
	id, err := age.ParseHybridIdentity(strings.TrimSpace(string(secret)))
	if err != nil {
		return nil, fmt.Errorf("parse identity %q: %w", name, err)
	}
	secrets[name] = id
	return id, nil
}
