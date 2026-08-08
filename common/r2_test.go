package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewR2StoreFromEnvRequiresCompleteConfiguration(t *testing.T) {
	t.Setenv("R2_ACCOUNT_ID", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_PUBLIC_BASE_URL", "")
	t.Setenv("R2_ENDPOINT", "")

	store, err := NewR2StoreFromEnv()
	require.NoError(t, err)
	assert.Nil(t, store)

	t.Setenv("R2_BUCKET", "temporary-assets")
	store, err = NewR2StoreFromEnv()
	require.Error(t, err)
	assert.Nil(t, store)

	t.Setenv("R2_ACCOUNT_ID", "account-id")
	t.Setenv("R2_ACCESS_KEY_ID", "access-key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret-key")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://file.lunadownload.com/")
	store, err = NewR2StoreFromEnv()
	require.NoError(t, err)
	require.NotNil(t, store)
	assert.Equal(t, "https://file.lunadownload.com/temporary/2026/08/08/file.jpg", store.URL("temporary/2026/08/08/file.jpg"))
}

func TestR2DatePrefix(t *testing.T) {
	assert.Equal(t, "temporary/2026/08/08/", R2DatePrefix(time.Date(2026, time.August, 8, 9, 10, 11, 0, time.UTC)))
}
