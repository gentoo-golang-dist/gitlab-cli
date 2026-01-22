package agentutils

import (
	"encoding/json"
	"errors"
	"log"
	"slices"

	"github.com/zalando/go-keyring"
)

const (
	keyringService      = "glab"
	inventoryKeyringKey = "agent-token-inventory"
)

// GetKeyringInventory returns the list of cached token IDs from the keyring inventory.
// Returns nil slice with no error if no inventory exists.
// Returns nil slice with error if keyring is unsupported or inventory is corrupted.
func GetKeyringInventory() ([]string, error) {
	data, err := keyring.Get(keyringService, inventoryKeyringKey)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var inventory []string
	if err := json.Unmarshal([]byte(data), &inventory); err != nil {
		errMsg := "keyring inventory is corrupted: " + err.Error()
		log.Println("Warning:", errMsg)
		return nil, errors.New(errMsg)
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

	// Check if ID already exists
	if slices.Contains(inventory, id) {
		return nil // Already in inventory
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

	originalLen := len(inventory)
	updated := slices.DeleteFunc(inventory, func(existingID string) bool {
		return existingID == id
	})

	// If nothing was removed, we're done
	if len(updated) == originalLen {
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
