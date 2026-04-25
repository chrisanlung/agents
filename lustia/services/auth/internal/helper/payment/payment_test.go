//go:build dev || local

package payment_test

import (
	"context"
	"testing"

	"github.com/chrisanlung/lustia-auth/internal/helper/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DummyMidtransClient tests
// ---------------------------------------------------------------------------

func TestNewDummy_AcceptsDevEnv(t *testing.T) {
	for _, env := range []string{"dev", "local"} {
		t.Run(env, func(t *testing.T) {
			d, err := payment.NewDummy(env)
			require.NoError(t, err)
			require.NotNil(t, d)
		})
	}
}

func TestNewDummy_RejectsNonDevEnv(t *testing.T) {
	for _, env := range []string{"staging", "production", "prod", ""} {
		t.Run(env, func(t *testing.T) {
			d, err := payment.NewDummy(env)
			assert.Error(t, err)
			assert.Nil(t, d)
		})
	}
}

func TestDummy_CreateTransaction_ReturnsPaid(t *testing.T) {
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	resp, err := d.CreateTransaction(context.Background(), payment.PaymentRequest{
		OrderID:     "B7K3-M2QF",
		GrossAmount: 150000,
	})
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPaid, resp.Status)
	assert.NotEmpty(t, resp.SnapToken)
}

func TestDummy_HandleNotification_ReturnsPaid(t *testing.T) {
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	status, err := d.HandleNotification(context.Background(), payment.WebhookNotification{
		OrderID:           "B7K3-M2QF",
		TransactionStatus: "settlement",
		GrossAmount:       "150000.00",
	})
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPaid, status)
}

func TestDummy_HandleNotification_ErrorOnMissingOrderID(t *testing.T) {
	d, err := payment.NewDummy("dev")
	require.NoError(t, err)

	_, err = d.HandleNotification(context.Background(), payment.WebhookNotification{
		OrderID: "", // missing
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestNewClient_DummyInDev(t *testing.T) {
	client, err := payment.NewClient(payment.Config{
		Adapter: "dummy",
		AppEnv:  "dev",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestNewClient_DummyInLocal(t *testing.T) {
	client, err := payment.NewClient(payment.Config{
		Adapter: "dummy",
		AppEnv:  "local",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

// C-2: factory must refuse dummy adapter outside dev/local.
func TestNewClient_DummyInProduction_ReturnsError(t *testing.T) {
	for _, env := range []string{"staging", "production", "prod", ""} {
		t.Run("APP_ENV="+env, func(t *testing.T) {
			client, err := payment.NewClient(payment.Config{
				Adapter: "dummy",
				AppEnv:  env,
			})
			assert.Error(t, err, "expected error for APP_ENV=%q", env)
			assert.Nil(t, client)
		})
	}
}

func TestNewClient_UnknownAdapter(t *testing.T) {
	_, err := payment.NewClient(payment.Config{
		Adapter: "stripe",
		AppEnv:  "dev",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ParseGrossAmount tests
// ---------------------------------------------------------------------------

func TestParseGrossAmount(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"150000.00", 150000, false},
		{"0.00", 0, false},
		{"999999999", 999999999, false},
		{"", 0, true},
		{"not-a-number", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := payment.ParseGrossAmount(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
