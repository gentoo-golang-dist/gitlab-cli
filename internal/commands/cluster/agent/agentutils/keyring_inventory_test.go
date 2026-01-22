package agentutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestGetKeyringInventory_NotFound(t *testing.T) {
	keyring.MockInit()

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestGetKeyringInventory_UnsupportedPlatform(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)

	ids, err := GetKeyringInventory()
	require.ErrorIs(t, err, keyring.ErrUnsupportedPlatform)
	assert.Nil(t, ids)
}

func TestGetKeyringInventory_Corrupted(t *testing.T) {
	keyring.MockInit()

	// Set corrupted JSON data
	err := keyring.Set(keyringService, inventoryKeyringKey, "not valid json")
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keyring inventory is corrupted")
	assert.Nil(t, ids)
}

func TestGetKeyringInventory_Valid(t *testing.T) {
	keyring.MockInit()

	// Set valid inventory
	err := keyring.Set(keyringService, inventoryKeyringKey, `["id1","id2","id3"]`)
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"id1", "id2", "id3"}, ids)
}

func TestAddToKeyringInventory_FirstEntry(t *testing.T) {
	keyring.MockInit()

	err := AddToKeyringInventory("new-id")
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"new-id"}, ids)
}

func TestAddToKeyringInventory_AdditionalEntry(t *testing.T) {
	keyring.MockInit()

	// Add first entry
	err := AddToKeyringInventory("id1")
	require.NoError(t, err)

	// Add second entry
	err = AddToKeyringInventory("id2")
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"id1", "id2"}, ids)
}

func TestAddToKeyringInventory_DuplicateNotAdded(t *testing.T) {
	keyring.MockInit()

	// Add entry
	err := AddToKeyringInventory("id1")
	require.NoError(t, err)

	// Try to add same entry again
	err = AddToKeyringInventory("id1")
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"id1"}, ids)
}

func TestAddToKeyringInventory_UnsupportedPlatform(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)

	err := AddToKeyringInventory("id1")
	require.ErrorIs(t, err, keyring.ErrUnsupportedPlatform)
}

func TestRemoveFromKeyringInventory_ExistingEntry(t *testing.T) {
	keyring.MockInit()

	// Set up inventory with entries
	err := keyring.Set(keyringService, inventoryKeyringKey, `["id1","id2","id3"]`)
	require.NoError(t, err)

	// Remove middle entry
	err = RemoveFromKeyringInventory("id2")
	require.NoError(t, err)

	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"id1", "id3"}, ids)
}

func TestRemoveFromKeyringInventory_NonExistentEntry(t *testing.T) {
	keyring.MockInit()

	// Set up inventory
	err := keyring.Set(keyringService, inventoryKeyringKey, `["id1","id2"]`)
	require.NoError(t, err)

	// Try to remove non-existent entry
	err = RemoveFromKeyringInventory("id3")
	require.NoError(t, err)

	// Inventory should be unchanged
	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Equal(t, []string{"id1", "id2"}, ids)
}

func TestRemoveFromKeyringInventory_LastEntry(t *testing.T) {
	keyring.MockInit()

	// Set up inventory with single entry
	err := keyring.Set(keyringService, inventoryKeyringKey, `["id1"]`)
	require.NoError(t, err)

	// Remove the last entry
	err = RemoveFromKeyringInventory("id1")
	require.NoError(t, err)

	// Inventory should be deleted (returns nil, nil)
	ids, err := GetKeyringInventory()
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestRemoveFromKeyringInventory_EmptyInventory(t *testing.T) {
	keyring.MockInit()

	// No inventory exists
	err := RemoveFromKeyringInventory("id1")
	require.NoError(t, err)
}

func TestRemoveFromKeyringInventory_UnsupportedPlatform(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)

	err := RemoveFromKeyringInventory("id1")
	require.ErrorIs(t, err, keyring.ErrUnsupportedPlatform)
}
