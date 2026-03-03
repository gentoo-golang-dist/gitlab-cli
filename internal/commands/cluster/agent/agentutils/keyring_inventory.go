package agentutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/zalando/go-keyring"
)

const (
	KeyringService      = "glab"
	inventoryKeyringKey = "agent-token-inventory"
)

// GetKeyringInventory returns the list of cached token IDs from the keyring inventory.
// Returns nil slice with no error if no inventory exists.
// Returns nil slice with error if keyring is unsupported or inventory is corrupted.
func GetKeyringInventory() ([]string, error) {
	data, err := keyring.Get(KeyringService, inventoryKeyringKey)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var inventory []string
	if err := json.Unmarshal([]byte(data), &inventory); err != nil {
		return nil, fmt.Errorf("keyring inventory is corrupted: %w", err)
	}

	return inventory, nil
}

// AddToKeyringInventory adds a token ID to the keyring inventory.
// If the ID already exists in the inventory, it is not added again.
func AddToKeyringInventory(id string) error {
	inventory, err := GetKeyringInventory()
	if err != nil {
		if errors.Is(err, keyring.ErrUnsupportedPlatform) {
			return err
		}
		// Start fresh if we can't read the inventory
		inventory = nil
	}

	if slices.Contains(inventory, id) {
		return nil
	}

	inventory = append(inventory, id)

	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	return keyring.Set(KeyringService, inventoryKeyringKey, string(data))
}

// RemoveFromKeyringInventory removes a token ID from the keyring inventory.
func RemoveFromKeyringInventory(id string) error {
	inventory, err := GetKeyringInventory()
	if err != nil {
		if errors.Is(err, keyring.ErrUnsupportedPlatform) {
			return err
		}
		return nil // Nothing to remove if we can't read
	}

	updated := slices.DeleteFunc(inventory, func(existingID string) bool {
		return existingID == id
	})

	switch len(updated) {
	case len(inventory):
		return nil
	case 0:
		err := keyring.Delete(KeyringService, inventoryKeyringKey)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
		return nil
	}

	data, err := json.Marshal(updated)
	if err != nil {
		return err
	}

	return keyring.Set(KeyringService, inventoryKeyringKey, string(data))
}
