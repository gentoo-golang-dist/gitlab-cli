package agentutils

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	keyringService      = "glab"
	inventoryKeyringKey = "agent-token-inventory"
)

// GetKeyringInventory returns the list of cached token IDs from the keyring inventory.
// Returns an empty slice if no inventory exists or keyring is unsupported.
func GetKeyringInventory() ([]string, error) {
	data, err := keyring.Get(keyringService, inventoryKeyringKey)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return []string{}, nil
		}
		if errors.Is(err, keyring.ErrUnsupportedPlatform) {
			return nil, err
		}
		return nil, err
	}

	var inventory []string
	if err := json.Unmarshal([]byte(data), &inventory); err != nil {
		// If the inventory is corrupted, return empty and let it be rebuilt
		return []string{}, nil
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
		inventory = []string{}
	}

	// Check if ID already exists
	for _, existingID := range inventory {
		if existingID == id {
			return nil // Already in inventory
		}
	}

	// Add new ID
	inventory = append(inventory, id)

	// Save updated inventory
	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	return keyring.Set(keyringService, inventoryKeyringKey, string(data))
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

	// Find and remove the ID
	var updated []string
	for _, existingID := range inventory {
		if existingID != id {
			updated = append(updated, existingID)
		}
	}

	// If nothing was removed, we're done
	if len(updated) == len(inventory) {
		return nil
	}

	// Save updated inventory
	if len(updated) == 0 {
		// Remove the inventory key entirely if empty
		err := keyring.Delete(keyringService, inventoryKeyringKey)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
		return nil
	}

	data, err := json.Marshal(updated)
	if err != nil {
		return err
	}

	return keyring.Set(keyringService, inventoryKeyringKey, string(data))
}
