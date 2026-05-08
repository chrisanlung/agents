package payment_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/helper/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// testHMACSHA256 computes HMAC-SHA256(key, data) → hex.
// Used in tests to pre-compute valid signatures for webhook fixture payloads
// without importing crypto packages at package level in ways that conflict.
// ---------------------------------------------------------------------------

func testHMACSHA256(data []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// buildSignedJSONWebhook constructs a JSON webhook payload with a valid
// HMAC-SHA256 signature (VA-keyed, over the payload without the signature field).
func buildSignedJSONWebhook(t *testing.T, fields map[string]interface{}, va string) []byte {
	t.Helper()

	// Step 1: make a copy without the "signature" key for HMAC input.
	unsigned := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		if k != "signature" {
			unsigned[k] = v
		}
	}

	// Step 2: encoding/json sorts map keys ascending — matches PHP ksort.
	jsonBytes, err := json.Marshal(unsigned)
	require.NoError(t, err)

	// Step 3: HMAC-SHA256 keyed by VA.
	sig := testHMACSHA256(jsonBytes, va)

	// Step 4: full payload with signature.
	fields["signature"] = sig
	full, err := json.Marshal(fields)
	require.NoError(t, err)
	return full
}

// ---------------------------------------------------------------------------
// NewIPaymu construction
// ---------------------------------------------------------------------------

func TestNewIPaymu_RequiredFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		va        string
		apiKey    string
		notifyURL string
		wantErr   string
	}{
		{"missing VA", "", "key", "https://notify.example.com", "IPAYMU_VA"},
		{"missing APIKey", "va", "", "https://notify.example.com", "IPAYMU_API_KEY"},
		{"missing NotifyURL", "va", "key", "", "IPAYMU_NOTIFY_URL"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := payment.NewIPaymu(tc.va, tc.apiKey, "", tc.notifyURL, false)
			require.Error(t, err)
			assert.Nil(t, p)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewIPaymu_DefaultBaseURL(t *testing.T) {
	t.Parallel()
	// Empty baseURL should succeed — defaults to https://sandbox.ipaymu.com.
	p, err := payment.NewIPaymu("va123", "apikey456", "", "https://notify.example.com", false)
	require.NoError(t, err)
	require.NotNil(t, p)
}

// ---------------------------------------------------------------------------
// Outbound signature — fixture test
//
// Verified against the reference algorithm in iPaymu Go SDK signature.go:
//   https://raw.githubusercontent.com/ipaymu/ipaymu-go-api/main/signature.go
//
// Algorithm:
//   bodyHash     = sha256(bodyBytes)
//   bodyHashHex  = hex.EncodeToString(bodyHash)   (lowercase)
//   stringToSign = "POST:" + va + ":" + bodyHashHex + ":" + apiKey
//   signature    = hmac_sha256(apiKey, stringToSign) → hex (lowercase)
//
// We test by sending a CreateQR to a local httptest server and verifying the
// captured "signature" header is:
//   (a) non-empty
//   (b) exactly 64 lowercase hex characters
//   (c) matches the expected value computed here independently.
// ---------------------------------------------------------------------------

func TestGenerateSignature_KnownFixture(t *testing.T) {
	t.Parallel()

	const (
		testVA     = "1179000899"
		testAPIKey = "TESTAPIKEY"
	)

	capturedSig := ""
	capturedBody := []byte(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("signature")
		// Read body so we can verify the signature independently.
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		capturedBody = make([]byte, n)
		copy(capturedBody, buf[:n])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status":  200,
			"Message": "OK",
			"Data": map[string]interface{}{
				"SessionId":     "ses123",
				"TransactionId": 99999,
				"ReferenceId":   "ref-fixture",
				"Via":           "qris",
				"Channel":       "qris",
				"PaymentNo":     "00020101fixture",
				"PaymentName":   "QRIS",
				"Total":         100000,
				"Fee":           0,
				"Expired":       "2099-01-01 00:00:00",
				"Note":          "",
				"Url":           "",
			},
		})
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu(testVA, testAPIKey, srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "ref-fixture",
		OrderID:           "TEST-001",
		AmountIDR:         100000,
		CustomerName:      "Test User",
		CustomerEmail:     "test@example.com",
		CustomerPhone:     "08123456789",
		Description:       "Test booking",
		ExpiryMinutes:     15,
	})
	require.NoError(t, err)
	require.NotEmpty(t, capturedSig, "signature header must be set")

	// Independently compute the expected signature over the captured body.
	bodyHash := sha256.Sum256(capturedBody)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	stringToSign := "POST:" + testVA + ":" + bodyHashHex + ":" + testAPIKey
	mac := hmac.New(sha256.New, []byte(testAPIKey))
	mac.Write([]byte(stringToSign))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expectedSig, capturedSig, "adapter signature must match independent calculation")
	assert.Regexp(t, `^[0-9a-f]{64}$`, capturedSig, "signature must be 64-char lowercase hex (HMAC-SHA256)")
}

// ---------------------------------------------------------------------------
// CreateQR — happy path and error cases
// ---------------------------------------------------------------------------

func TestIPaymu_CreateQR_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/payment/direct", r.URL.Path)
		assert.Equal(t, "testVA", r.Header.Get("va"))
		assert.NotEmpty(t, r.Header.Get("signature"))

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "qris", body["paymentMethod"])
		assert.Equal(t, "qris", body["paymentChannel"])
		assert.Equal(t, "my-ref-001", body["referenceId"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status":  200,
			"Message": "OK",
			"Data": map[string]interface{}{
				"SessionId":     "ses-abc",
				"TransactionId": int64(12345678),
				"ReferenceId":   "my-ref-001",
				"Via":           "qris",
				"Channel":       "qris",
				"PaymentNo":     "00020101qrspayload",
				"PaymentName":   "QRIS",
				"Total":         int64(385000),
				"Fee":           0,
				"Expired":       "2099-12-31 23:59:59",
				"Note":          "",
				"Url":           "https://sandbox.ipaymu.com/qr/test.png",
			},
		})
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("testVA", "testAPIKey", srv.URL, "https://notify.ngrok.example.com/webhook", false)
	require.NoError(t, err)

	resp, err := p.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "my-ref-001",
		OrderID:           "AB12-CD34",
		AmountIDR:         385000,
		CustomerName:      "Budi Santoso",
		CustomerEmail:     "budi@example.com",
		CustomerPhone:     "081234567890",
		Description:       "Spa booking",
		ExpiryMinutes:     15,
	})
	require.NoError(t, err)

	// ProviderReference must be OUR UUID reference, not iPaymu's TransactionId.
	assert.Equal(t, "my-ref-001", resp.ProviderReference)
	assert.Equal(t, "00020101qrspayload", resp.QRString)
	assert.Equal(t, "https://sandbox.ipaymu.com/qr/test.png", resp.QRImageURL)
	assert.True(t, resp.ExpiresAt.After(time.Now()), "ExpiresAt must be in the future")
}

func TestIPaymu_CreateQR_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status":  400,
			"Message": "Invalid virtual account",
			"Data":    nil,
		})
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("badVA", "badKey", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "ref-err",
		OrderID:           "XX-YY",
		AmountIDR:         10000,
		ExpiryMinutes:     15,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid virtual account")
}

func TestIPaymu_CreateQR_EmptyRef_ReturnsError(t *testing.T) {
	t.Parallel()

	p, err := payment.NewIPaymu("va", "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "",
		OrderID:           "AB12",
		AmountIDR:         10000,
	})
	require.Error(t, err)
}

func TestIPaymu_CreateQR_FallbackExpiry_WhenExpiredEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status":  200,
			"Message": "OK",
			"Data": map[string]interface{}{
				"PaymentNo": "QRPAYLOAD",
				"Expired":   "", // empty — should trigger fallback
				"Url":       "",
			},
		})
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	before := time.Now()
	resp, err := p.CreateQR(context.Background(), payment.CreateQRRequest{
		ProviderReference: "ref-fallback",
		OrderID:           "AB12",
		AmountIDR:         50000,
		ExpiryMinutes:     15,
	})
	require.NoError(t, err)

	// Fallback: ExpiresAt must be approximately 15 minutes from now.
	assert.True(t, resp.ExpiresAt.After(before.Add(14*time.Minute)),
		"ExpiresAt must be at least 14 min from now, got %s", resp.ExpiresAt)
	assert.True(t, resp.ExpiresAt.Before(before.Add(16*time.Minute)),
		"ExpiresAt must be at most 16 min from now, got %s", resp.ExpiresAt)
}

// ---------------------------------------------------------------------------
// VerifyWebhook — signature verification
// ---------------------------------------------------------------------------

func TestIPaymu_VerifyWebhook_ValidSignatureJSON(t *testing.T) {
	t.Parallel()
	const testVA = "testVA123"

	fields := map[string]interface{}{
		"trx_id":          "999",
		"reference_id":    "our-uuid-ref",
		"sid":             "ses-abc",
		"status":          "berhasil",
		"status_code":     "1",
		"amount":          "385000",
		"paid_off":        "385000",
		"payment_method":  "qris",
		"payment_channel": "qris",
	}
	payload := buildSignedJSONWebhook(t, fields, testVA)

	p, err := payment.NewIPaymu(testVA, "apikey", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	notif, err := p.VerifyWebhook(context.Background(), payload, map[string]string{
		"Content-Type": "application/json",
	})
	require.NoError(t, err)
	assert.Equal(t, "our-uuid-ref", notif.ProviderReference)
	assert.Equal(t, payment.StatusPaid, notif.Status)
	assert.Equal(t, int64(385000), notif.ReceivedAmountIDR)
	// RawPayload re-marshalled (form-urlencoded → JSON for JSONB column),
	// signature field stripped — must not contain credential.
	assert.NotContains(t, string(notif.RawPayload), "signature")
}

func TestIPaymu_VerifyWebhook_InvalidSignature_Rejected(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"trx_id":"1","reference_id":"ref","status_code":"1","paid_off":"100","signature":"badhex0000000000000000000000000000000000000000000000000000000000"}`)

	p, err := payment.NewIPaymu("testVA123", "apikey", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.VerifyWebhook(context.Background(), payload, map[string]string{
		"Content-Type": "application/json",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature mismatch")
}

func TestIPaymu_VerifyWebhook_MissingSignature_Rejected(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"trx_id":"1","reference_id":"ref","status_code":"1","paid_off":"100"}`)

	p, err := payment.NewIPaymu("va", "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.VerifyWebhook(context.Background(), payload, map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature missing")
}

func TestIPaymu_VerifyWebhook_SignatureInXHeader(t *testing.T) {
	t.Parallel()
	const testVA = "headerVA"

	// Build payload WITHOUT the "signature" field in the body.
	fields := map[string]interface{}{
		"trx_id":       "10",
		"reference_id": "hdr-ref",
		"status_code":  "1",
		"paid_off":     "50000",
	}
	jsonBytes, err := json.Marshal(fields)
	require.NoError(t, err)
	sig := testHMACSHA256(jsonBytes, testVA)

	p, err := payment.NewIPaymu(testVA, "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	// Signature is in the X-Signature header instead of the body.
	notif, err := p.VerifyWebhook(context.Background(), jsonBytes, map[string]string{
		"Content-Type": "application/json",
		"X-Signature":  sig,
	})
	require.NoError(t, err)
	assert.Equal(t, "hdr-ref", notif.ProviderReference)
}

func TestIPaymu_VerifyWebhook_StatusMapping(t *testing.T) {
	t.Parallel()
	const testVA = "mapVA"

	cases := []struct {
		name       string
		statusCode string
		statusStr  string
		want       payment.PaymentStatus
	}{
		{"success_code", "1", "berhasil", payment.StatusPaid},
		{"pending_code", "0", "pending", payment.StatusPending},
		{"expired_code", "-2", "expired", payment.StatusExpired},
		{"cancel_code", "2", "cancel", payment.StatusFailed},
		{"failed_code", "5", "failed", payment.StatusFailed},
		// Fallback to status string when status_code is empty.
		{"berhasil_str", "", "berhasil", payment.StatusPaid},
		{"pending_str", "", "pending", payment.StatusPending},
		{"expired_str", "", "expired", payment.StatusExpired},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fields := map[string]interface{}{
				"trx_id":          "1",
				"reference_id":    "ref-map",
				"status":          tc.statusStr,
				"status_code":     tc.statusCode,
				"amount":          "10000",
				"paid_off":        "10000",
				"payment_method":  "qris",
				"payment_channel": "qris",
			}
			payload := buildSignedJSONWebhook(t, fields, testVA)

			p, err := payment.NewIPaymu(testVA, "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
			require.NoError(t, err)
			notif, err := p.VerifyWebhook(context.Background(), payload, map[string]string{"Content-Type": "application/json"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, notif.Status)
		})
	}
}

func TestIPaymu_VerifyWebhook_FormEncoded(t *testing.T) {
	t.Parallel()
	const testVA = "formVA"

	// For form-encoded payloads the map values are all strings.
	fields := map[string]interface{}{
		"trx_id":          "42",
		"reference_id":    "form-ref",
		"status":          "berhasil",
		"status_code":     "1",
		"amount":          "200000",
		"paid_off":        "200000",
		"payment_method":  "qris",
		"payment_channel": "qris",
	}
	jsonBytes, err := json.Marshal(fields)
	require.NoError(t, err)
	sig := testHMACSHA256(jsonBytes, testVA)

	// Build form-encoded body.
	var parts []string
	for k, v := range fields {
		parts = append(parts, k+"="+v.(string))
	}
	parts = append(parts, "signature="+sig)
	formPayload := []byte(strings.Join(parts, "&"))

	p, err := payment.NewIPaymu(testVA, "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	notif, err := p.VerifyWebhook(context.Background(), formPayload, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	require.NoError(t, err)
	assert.Equal(t, "form-ref", notif.ProviderReference)
	assert.Equal(t, payment.StatusPaid, notif.Status)
	assert.Equal(t, int64(200000), notif.ReceivedAmountIDR)
}

// ---------------------------------------------------------------------------
// GetStatus — real implementation tests
// ---------------------------------------------------------------------------

// buildCheckResponse is a test helper that serialises a transaction/check
// response envelope for use in httptest handlers.
func buildCheckResponse(t *testing.T, outerStatus int, message string, dataStatus int) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"Status":  outerStatus,
		"Message": message,
		"Data": map[string]interface{}{
			"TransactionId": int64(12345),
			"SessionId":     "ses-abc",
			"ReferenceId":   "test-ref",
			"Amount":        int64(385000),
			"Fee":           int64(2695),
			"Status":        dataStatus,
			"StatusDesc":    "test",
		},
	})
	require.NoError(t, err)
	return body
}

func TestIPaymu_GetStatus_HappyPath(t *testing.T) {
	t.Parallel()

	var capturedSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/transaction/check", r.URL.Path)
		capturedSig = r.Header.Get("signature")
		assert.NotEmpty(t, capturedSig)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildCheckResponse(t, 200, "OK", 1)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("testVA", "testKey", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	status, err := p.GetStatus(context.Background(), "my-uuid-ref")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPaid, status)
	assert.Regexp(t, `^[0-9a-f]{64}$`, capturedSig, "signature header must be 64-char lowercase hex")
}

func TestIPaymu_GetStatus_PendingStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildCheckResponse(t, 200, "OK", 0)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	status, err := p.GetStatus(context.Background(), "ref-pending")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPending, status)
}

func TestIPaymu_GetStatus_ExpiredStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildCheckResponse(t, 200, "OK", -2)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	status, err := p.GetStatus(context.Background(), "ref-expired")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusExpired, status)
}

func TestIPaymu_GetStatus_FailedStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildCheckResponse(t, 200, "OK", 5)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	status, err := p.GetStatus(context.Background(), "ref-failed")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusFailed, status)
}

func TestIPaymu_GetStatus_UnknownStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Data.Status=99 — unrecognised code. Must not crash; returns StatusFailed.
		w.Write(buildCheckResponse(t, 200, "OK", 99)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	status, err := p.GetStatus(context.Background(), "ref-unknown")
	require.NoError(t, err)
	assert.Equal(t, payment.StatusFailed, status)
}

func TestIPaymu_GetStatus_OuterEnvelopeFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(map[string]interface{}{
			"Status":  400,
			"Message": "invalid signature",
			"Data":    nil,
		})
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.GetStatus(context.Background(), "ref-bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestIPaymu_GetStatus_NetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close the connection abruptly to simulate a network error.
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.GetStatus(context.Background(), "ref-net-error")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ipaymu")
}

func TestIPaymu_GetStatus_EmptyReferenceRejected(t *testing.T) {
	t.Parallel()

	// No HTTP server needed — error must be returned before any outbound call.
	p, err := payment.NewIPaymu("va", "key", "https://sandbox.ipaymu.com", "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.GetStatus(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "providerReference must not be empty")
}

func TestIPaymu_GetStatus_RequestBodyContainsReferenceId(t *testing.T) {
	t.Parallel()

	const wantRef = "my-booking-uuid-123"
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildCheckResponse(t, 200, "OK", 1)) //nolint:errcheck
	}))
	defer srv.Close()

	p, err := payment.NewIPaymu("va", "key", srv.URL, "https://notify.example.com", false)
	require.NoError(t, err)

	_, err = p.GetStatus(context.Background(), wantRef)
	require.NoError(t, err)

	var bodyMap map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &bodyMap))
	assert.Equal(t, wantRef, bodyMap["referenceId"], "body must carry referenceId with our UUID")
	_, hasTransactionId := bodyMap["transactionId"]
	assert.False(t, hasTransactionId, "body must NOT contain transactionId")
}

func TestIPaymu_ListSettlements_ReturnsNotImplemented(t *testing.T) {
	t.Parallel()
	p, err := payment.NewIPaymu("va", "key", "", "https://notify.example.com", false)
	require.NoError(t, err)
	_, err = p.ListSettlements(context.Background(), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// ---------------------------------------------------------------------------
// Factory — iPaymu path
// ---------------------------------------------------------------------------

func TestNewProvider_IPaymu_MissingNotifyURL_FailsFast(t *testing.T) {
	t.Parallel()
	_, err := payment.NewProvider(payment.ProviderConfig{
		Provider:        "ipaymu",
		AppEnv:          "dev",
		IPaymuVA:        "va123",
		IPaymuAPIKey:    "key123",
		IPaymuBaseURL:   "https://sandbox.ipaymu.com",
		IPaymuNotifyURL: "", // must fail fast
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IPAYMU_NOTIFY_URL")
}

func TestNewProvider_IPaymu_ValidConfig_Succeeds(t *testing.T) {
	t.Parallel()
	p, err := payment.NewProvider(payment.ProviderConfig{
		Provider:        "ipaymu",
		AppEnv:          "dev",
		IPaymuVA:        "va123",
		IPaymuAPIKey:    "key123",
		IPaymuBaseURL:   "https://sandbox.ipaymu.com",
		IPaymuNotifyURL: "https://example.ngrok-free.app/api/v1/public/payments/webhook",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}
