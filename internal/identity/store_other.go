//go:build !darwin

package identity

import (
	"errors"

	"filippo.io/age"
)

var errUnavailable = errors.New("iCloud Keychain is only available on macOS")

func Generate(name string, mode AccessControlMode) (string, error) {
	return "", errUnavailable
}

func List() ([]Item, error) {
	return nil, errUnavailable
}

func Delete(name string) error {
	return errUnavailable
}

func LoadIdentity(name string) (age.Identity, error) {
	return nil, errUnavailable
}

func LoadRecipient(name string) (age.Recipient, error) {
	return nil, errUnavailable
}
