//go:build dev || local

package payment_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/helper/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DummyProvider tests
// ---------------------------------------------------------------------------

func TestNewDummy_AcceptsDevEnv(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"dev", "local"} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			d, err := payment.NewDummy(env)
			require.NoError(t, err)
			require.NotNil(t, d)
		})
	}
}

func TestNewDummy_RejectsNonDevEnv(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"staging", "production", "prod", ""} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			d, err := payment.NewDummy(env)
			assert.Error(t, err)
			assert.Nil(t, d)
		})
	}
}

func TestDummy_CreateQR_ReturnsQRString(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	resp, err := d.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "test-ref-001",
		OrderID:           "AB12-CD34",
		AmountIDR:         150000,
		ExpiryMinutes:     15,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.QRString)
	assert.NotEmpty(t, resp.QRImageURL)
	assert.Equal(t, "test-ref-001", resp.ProviderReference)
	assert.True(t, resp.ExpiresAt.After(time.Now()))
}

func TestDummy_CreateQR_ErrorOnEmptyRef(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	_, err = d.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "",
		OrderID:           "AB12-CD34",
		AmountIDR:         150000,
	})
	assert.Error(t, err)
}

func TestDummy_CreateQR_DoesNotAutoPayPhase6(t *testing.T) {
	t.Parallel()
	// Phase 6: CreateQR must NOT return Status=paid. Caller must trigger webhook.
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	resp, err := d.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "ref-phase6",
		OrderID:           "AB12-CD34",
		AmountIDR:         100000,
		ExpiryMinutes:     15,
	})
	require.NoError(t, err)
	// ProviderReference echoed back — service layer decides when to mark paid.
	assert.Equal(t, "ref-phase6", resp.ProviderReference)
	// No status field on CreateQRResponse — that is the point. No auto-paid.
}

func TestDummy_VerifyWebhook_ParsesPayload(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]interface{}{
		"provider_reference": "ref-abc",
		"amount_idr":         int64(150000),
	})
	notif, err := d.VerifyWebhook(context.Background(), payload, nil)
	require.NoError(t, err)
	assert.Equal(t, "ref-abc", notif.ProviderReference)
	assert.Equal(t, payment.StatusPaid, notif.Status)
	assert.Equal(t, int64(150000), notif.ReceivedAmountIDR)
}

func TestDummy_VerifyWebhook_ErrorOnEmptyPayload(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	_, err = d.VerifyWebhook(context.Background(), nil, nil)
	assert.Error(t, err)
}

func TestDummy_GetStatus_ReturnsPaid(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	status, err := d.GetStatus(context.Background(), "ref-xyz")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPaid, status)
}

func TestDummy_ListSettlements_ReturnsSyntheticItem(t *testing.T) {
	t.Parallel()
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	items, err := d.ListSettlements(context.Background(), time.Now())
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestNewProvider_DummyInDev(t *testing.T) {
	t.Parallel()
	p, err := payment.NewProvider(payment.ProviderConfig{
		Provider: "dummy",
		AppEnv:   "dev",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewProvider_DummyInLocal(t *testing.T) {
	t.Parallel()
	p, err := payment.NewProvider(payment.ProviderConfig{
		Provider: "dummy",
		AppEnv:   "local",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

// C-2: factory must refuse dummy adapter outside dev/local.
func TestNewProvider_DummyInProduction_ReturnsError(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"staging", "production", "prod", ""} {
		env := env
		t.Run("APP_ENV="+env, func(t *testing.T) {
			t.Parallel()
			p, err := payment.NewProvider(payment.ProviderConfig{
				Provider: "dummy",
				AppEnv:   env,
			})
			assert.Error(t, err, "expected error for APP_ENV=%q", env)
			assert.Nil(t, p)
		})
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	t.Parallel()
	_, err := payment.NewProvider(payment.ProviderConfig{
		Provider: "stripe",
		AppEnv:   "dev",
	})
	assert.Error(t, err)
}
