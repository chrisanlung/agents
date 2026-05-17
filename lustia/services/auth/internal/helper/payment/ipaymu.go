package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IPaymuProvider is the iPaymu payment adapter (Phase 6, QRIS only).
//
// iPaymu is a QRIS-based Indonesian payment aggregator (ADR 0015).
// This adapter handles:
//   - CreateQR: POST /api/v2/payment/direct with HMAC-SHA256 signature.
//   - VerifyWebhook: verify HMAC-SHA256 (VA-keyed) + parse notification.
//   - GetStatus: STUB — returns errIPaymuNotImplemented; TODO Phase 7.
//   - ListSettlements: STUB — returns errIPaymuNotImplemented; manual in Phase 6.
//
// Credentials read from environment via factory.go:
//
//	IPAYMU_VA          — Virtual Account number (outbound header + inbound HMAC key)
//	IPAYMU_API_KEY     — API key (outbound HMAC signing key)
//	IPAYMU_BASE_URL    — defaults to https://sandbox.ipaymu.com
//	IPAYMU_NOTIFY_URL  — required; ngrok URL for webhook delivery in sandbox
type IPaymuProvider struct {
	va                  string
	apiKey              string
	baseURL             string
	notifyURL           string
	skipSignatureVerify bool
	client              *http.Client
}

// errIPaymuNotImplemented is returned by stubbed methods.
var errIPaymuNotImplemented = errors.New(
	"ipaymu: method not yet implemented — see TODO comments",
)

// jakartaLocation is Asia/Jakarta (WIB, UTC+7). Used to parse iPaymu's
// "2006-01-02 15:04:05" timestamp strings which are expressed in local time.
var jakartaLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback: construct a fixed-offset zone if the timezone DB is absent
		// (stripped Docker images). UTC+7 is always correct for WIB.
		loc = time.FixedZone("WIB", 7*60*60)
	}
	jakartaLocation = loc
}

// NewIPaymu constructs an IPaymuProvider.
// va, apiKey, notifyURL are required; baseURL defaults to sandbox if empty.
func NewIPaymu(va, apiKey, baseURL, notifyURL string, skipSignatureVerify bool) (*IPaymuProvider, error) {
	if va == "" {
		return nil, errors.New("ipaymu: IPAYMU_VA is required")
	}
	if apiKey == "" {
		return nil, errors.New("ipaymu: IPAYMU_API_KEY is required")
	}
	if notifyURL == "" {
		return nil, errors.New("ipaymu: IPAYMU_NOTIFY_URL is required — set to your ngrok URL in sandbox")
	}
	if baseURL == "" {
		baseURL = "https://sandbox.ipaymu.com"
	}
	slog.Info("ipaymu adapter constructed",
		"base_url", baseURL,
		"notify_url", notifyURL,
		"skip_signature_verify", skipSignatureVerify,
	)
	return &IPaymuProvider{
		va:                  va,
		apiKey:              apiKey,
		baseURL:             strings.TrimRight(baseURL, "/"),
		notifyURL:           notifyURL,
		skipSignatureVerify: skipSignatureVerify,
		client:              &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ---------------------------------------------------------------------------
// CreateQR — POST /api/v2/payment/direct
// ---------------------------------------------------------------------------

// ipaymuDirectRequest is the JSON body for the iPaymu direct payment endpoint.
// Field names and types follow the Go SDK (github.com/ipaymu/ipaymu-go-api).
type ipaymuDirectRequest struct {
	Name          string   `json:"name"`
	Phone         string   `json:"phone"`
	Email         string   `json:"email"`
	Amount        int64    `json:"amount"`
	NotifyURL     string   `json:"notifyUrl"`
	Expired       int      `json:"expired"`
	ExpiredType   string   `json:"expiredType"`
	Comments      string   `json:"comments"`
	ReferenceID   string   `json:"referenceId"`
	PaymentMethod string   `json:"paymentMethod"`
	PaymentChannel string  `json:"paymentChannel"`
	Product       []string `json:"product"`
	Qty           []int    `json:"qty"`
	Price         []int64  `json:"price"`
}

// ipaymuDirectResponse mirrors the iPaymu API response shape (Go SDK response.go).
type ipaymuDirectResponse struct {
	Status  int    `json:"Status"`
	Message string `json:"Message"`
	Data    struct {
		SessionID     string `json:"SessionId"`
		TransactionID int64  `json:"TransactionId"`
		ReferenceID   string `json:"ReferenceId"`
		Via           string `json:"Via"`
		Channel       string `json:"Channel"`
		// PaymentNo is the QRIS payload string that qr_flutter renders on device.
		PaymentNo   string `json:"PaymentNo"`
		PaymentName string `json:"PaymentName"`
		Total       int64  `json:"Total"`
		Fee         int64  `json:"Fee"`
		// Expired is the QR expiry timestamp from iPaymu, formatted as
		// "2006-01-02 15:04:05" in Asia/Jakarta (WIB) time.
		Expired string `json:"Expired"`
		Note    string `json:"Note"`
		// Url is an optional provider-hosted QR image URL. May be empty.
		Url string `json:"Url"`
	} `json:"Data"`
}

// CreateQR initiates a QRIS payment transaction via iPaymu.
//
// ProviderReference design: we keep req.ProviderReference (our UUID) as the
// returned ProviderReference. Rationale: InitiateForBooking stores
// qrResp.ProviderReference as payment_transaction.provider_reference, and
// HandleWebhook does FindByProviderReference to credit the row. iPaymu echoes
// our referenceId back as reference_id in the webhook notification body, so
// using our UUID as the lookup key is correct end-to-end. iPaymu's own
// TransactionId is not returned — it is not needed for QRIS correlation.
func (p *IPaymuProvider) CreateQR(ctx context.Context, req CreateQRRequest) (CreateQRResponse, error) {
	if req.ProviderReference == "" {
		return CreateQRResponse{}, errors.New("ipaymu: provider_reference is required")
	}

	expiryMinutes := req.ExpiryMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 15 // ADR 0015 §2.7 hard cap
	}

	channel := req.Channel
	if channel == "" {
		channel = "qris" // backwards-compat default
	}
	method, bankCode, err := resolveIPaymuChannel(channel)
	if err != nil {
		return CreateQRResponse{}, err
	}

	body := ipaymuDirectRequest{
		Name:           req.CustomerName,
		Phone:          sanitizePhone(req.CustomerPhone),
		Email:          req.CustomerEmail,
		Amount:         req.AmountIDR,
		NotifyURL:      p.notifyURL,
		Expired:        expiryMinutes,
		ExpiredType:    "minutes",
		Comments:       req.Description,
		ReferenceID:    req.ProviderReference,
		PaymentMethod:  method,
		PaymentChannel: bankCode,
		Product:        []string{fmt.Sprintf("Booking %s", req.OrderID)},
		Qty:            []int{1},
		Price:          []int64{req.AmountIDR},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: marshal request: %w", err)
	}

	sig, err := generateSignature(bodyBytes, p.va, p.apiKey)
	if err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: generate signature: %w", err)
	}

	endpoint := p.baseURL + "/api/v2/payment/direct"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("va", p.va)
	httpReq.Header.Set("signature", sig)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: read response body: %w", err)
	}

	var apiResp ipaymuDirectResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: parse response (http=%d): %w", httpResp.StatusCode, err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 || apiResp.Status != 200 {
		return CreateQRResponse{}, fmt.Errorf("ipaymu: API error (http=%d status=%d): %s",
			httpResp.StatusCode, apiResp.Status, apiResp.Message)
	}

	expiresAt := parseIPaymuExpiry(ctx, apiResp.Data.Expired, expiryMinutes)

	resp := CreateQRResponse{
		// Keep our own reference — see function doc above for rationale.
		ProviderReference: req.ProviderReference,
		Channel:           channel,
		ExpiresAt:         expiresAt,
	}
	if method == "qris" {
		resp.QRString = apiResp.Data.PaymentNo
		resp.QRImageURL = apiResp.Data.Url
	} else {
		// VA: iPaymu returns the VA number in PaymentNo for the chosen bank.
		resp.VANumber = apiResp.Data.PaymentNo
		resp.VABank = bankCode
	}
	return resp, nil
}

// resolveIPaymuChannel maps our Channel string ("qris" | "va_<bank>") to the
// iPaymu API pair (paymentMethod, paymentChannel).
//
// iPaymu accepts:
//
//	paymentMethod=qris  paymentChannel=qris
//	paymentMethod=va    paymentChannel=bca|mandiri|bni|bri|permata|cimb
//
// Returns method, channel (bank code or "qris"), or an error for unsupported
// channel strings.
func resolveIPaymuChannel(channel string) (method string, bankCode string, err error) {
	switch channel {
	case "qris":
		return "qris", "qris", nil
	case "va_bca":
		return "va", "bca", nil
	case "va_mandiri":
		return "va", "mandiri", nil
	case "va_bni":
		return "va", "bni", nil
	case "va_bri":
		return "va", "bri", nil
	case "va_permata":
		return "va", "permata", nil
	case "va_cimb":
		return "va", "cimb", nil
	default:
		return "", "", fmt.Errorf("ipaymu: unsupported channel %q (allowed: qris, va_bca, va_mandiri, va_bni, va_bri, va_permata, va_cimb)", channel)
	}
}

// ---------------------------------------------------------------------------
// VerifyWebhook — inbound POST from iPaymu
// ---------------------------------------------------------------------------

// ipaymuWebhookBody captures the fields iPaymu sends in its webhook notification.
// The Content-Type is either application/json or application/x-www-form-urlencoded
// (observed in iPaymu WooCommerce plugin, lines 320–700). Both are handled.
type ipaymuWebhookBody struct {
	TrxID          string `json:"trx_id"`
	ReferenceID    string `json:"reference_id"`
	Sid            string `json:"sid"`
	Status         string `json:"status"`
	StatusCode     string `json:"status_code"`
	Total          string `json:"total"`
	Amount         string `json:"amount"`
	PaidOff        string `json:"paid_off"`
	Fee            string `json:"fee"`
	PaymentNo      string `json:"payment_no"`
	PaymentMethod  string `json:"payment_method"`
	PaymentChannel string `json:"payment_channel"`
	Signature      string `json:"signature"`
}

// VerifyWebhook validates the iPaymu inbound webhook signature and maps the
// payload to a PaymentNotification.
//
// Signature algorithm (iPaymu WooCommerce plugin, lines 642–700):
//  1. Parse body fields into a map[string]interface{}.
//  2. Drop the "signature" key.
//  3. Sort keys ascending and JSON-encode — encoding/json already produces
//     ascending key order for map[string]interface{}, matching PHP ksort+json_encode.
//  4. expected = hmac_sha256(VA, json_string) → hex.
//  5. Compare with received signature (from body field OR X-Signature header).
//
// Note: the inbound HMAC key is the VA number, not the API key. This is
// unusual but intentional — verified in iPaymu's WooCommerce plugin source.
func (p *IPaymuProvider) VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (PaymentNotification, error) {
	if len(payload) == 0 {
		return PaymentNotification{}, errors.New("ipaymu: empty webhook payload")
	}

	// Detect content type from headers.
	contentType := ""
	for k, v := range headers {
		if strings.EqualFold(k, "content-type") {
			contentType = strings.ToLower(v)
			break
		}
	}

	// Parse payload into a generic map for signature verification and a typed
	// struct for field extraction. Both JSON and form-encoded are supported.
	rawMap, typed, err := parseWebhookPayload(payload, contentType)
	if err != nil {
		return PaymentNotification{}, fmt.Errorf("ipaymu: parse webhook payload: %w", err)
	}

	// Extract signature — prefer body field, fall back to X-Signature header.
	receivedSig := typed.Signature
	if receivedSig == "" {
		for k, v := range headers {
			if strings.EqualFold(k, "x-signature") {
				receivedSig = v
				break
			}
		}
	}
	if receivedSig == "" && !p.skipSignatureVerify {
		return PaymentNotification{}, errors.New("ipaymu: webhook signature missing")
	}

	// Compute expected signature over the payload map minus the signature field.
	delete(rawMap, "signature")
	expectedSig, err := computeWebhookSignature(rawMap, p.va)
	if err != nil && !p.skipSignatureVerify {
		return PaymentNotification{}, fmt.Errorf("ipaymu: compute webhook signature: %w", err)
	}

	if !hmac.Equal([]byte(receivedSig), []byte(expectedSig)) {
		if !p.skipSignatureVerify {
			return PaymentNotification{}, errors.New("ipaymu: webhook signature mismatch — rejecting payload")
		}
		slog.WarnContext(ctx,
			"ipaymu: webhook signature mismatch (skip enabled, accepting payload)",
			"received", receivedSig,
			"expected", expectedSig,
			"reference_id", typed.ReferenceID,
		)
	}

	// Map status to our normalised PaymentStatus.
	status := mapWebhookStatus(typed.StatusCode, typed.Status)

	// Use gross customer payment for the amount-mismatch guard. iPaymu QRIS
	// settles `paid_off = total - fee` to the merchant, but the customer paid
	// `total` in full — the gateway fee is a separate accounting concern. Order
	// of preference: total → amount → paid_off (for older sandbox payloads
	// that may omit total).
	receivedAmount := parseAmountString(typed.Total)
	if receivedAmount == 0 {
		receivedAmount = parseAmountString(typed.Amount)
	}
	if receivedAmount == 0 {
		receivedAmount = parseAmountString(typed.PaidOff)
	}

	// Persist payload as JSON in payment_transaction.raw_webhook (JSONB column).
	// iPaymu sandbox sends form-urlencoded which Postgres jsonb rejects, so we
	// marshal the parsed map. Falls back to original bytes if marshal fails.
	jsonPayload, marshalErr := json.Marshal(rawMap)
	if marshalErr != nil {
		jsonPayload = payload
	}

	return PaymentNotification{
		// reference_id echoes what we set as referenceId on CreateQR — our UUID.
		// HandleWebhook.FindByProviderReference uses this to look up the row.
		ProviderReference: typed.ReferenceID,
		Status:            status,
		ReceivedAmountIDR: receivedAmount,
		RawPayload:        jsonPayload,
	}, nil
}

// ---------------------------------------------------------------------------
// GetStatus — POST /api/v2/transaction/check
// ---------------------------------------------------------------------------

// ipaymuTransactionCheckRequest is the JSON body for the iPaymu
// transaction-check endpoint. We use referenceId (our UUID) rather than
// transactionId (iPaymu's numeric ID) because provider_reference stores our
// UUID, not iPaymu's TransactionId. This keeps the round-trip clean without
// requiring an extra column.
type ipaymuTransactionCheckRequest struct {
	ReferenceID string `json:"referenceId"`
}

// ipaymuTransactionCheckResponse mirrors the iPaymu API response for
// POST /api/v2/transaction/check (Go SDK ResponseCheck struct).
// Only fields we consume are declared; unknown fields are silently ignored.
type ipaymuTransactionCheckResponse struct {
	Status  int    `json:"Status"`
	Message string `json:"Message"`
	Data    struct {
		TransactionID  int64  `json:"TransactionId"`
		SessionID      string `json:"SessionId"`
		ReferenceID    string `json:"ReferenceId"`
		Amount         int64  `json:"Amount"`
		Fee            int64  `json:"Fee"`
		Status         int    `json:"Status"`
		StatusDesc     string `json:"StatusDesc"`
		Type           int    `json:"Type"`
		TypeDesc       string `json:"TypeDesc"`
		Notes          string `json:"Notes"`
		CreatedDate    string `json:"CreatedDate"`
		ExpiredDate    string `json:"ExpiredDate"`
		SuccessDate    string `json:"SuccessDate"`
		SettlementDate string `json:"SettlementDate"`
	} `json:"Data"`
}

// GetStatus polls iPaymu for the current transaction state using the
// POST /api/v2/transaction/check endpoint.
//
// We identify the transaction by referenceId (our caller-assigned UUID stored
// in payment_transaction.provider_reference), not by iPaymu's numeric
// TransactionId. iPaymu echoes our referenceId back in both the check response
// and webhook notifications, so the UUID is the correct correlation key
// end-to-end.
//
// Status mapping (from iPaymu Go SDK constants):
//
//	 1 (Berhasil/Success) → StatusPaid
//	 0 (Pending)          → StatusPending
//	-2 (Expired)          → StatusExpired
//	 2 (Cancel) / 5 (Failed) / 4 (Error) → StatusFailed
//	 anything else        → StatusFailed (logged at warn)
//
// Called by service.SyncStatus on behalf of the operator "Tarik Status
// Pembayaran" button (POST /api/v1/tenant/bookings/:id/sync-payment).
func (p *IPaymuProvider) GetStatus(ctx context.Context, providerReference string) (PaymentStatus, error) {
	if providerReference == "" {
		return StatusFailed, errors.New("ipaymu: providerReference must not be empty")
	}

	reqBody := ipaymuTransactionCheckRequest{ReferenceID: providerReference}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: marshal transaction/check request: %w", err)
	}

	sig, err := generateSignature(bodyBytes, p.va, p.apiKey)
	if err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: generate signature for transaction/check: %w", err)
	}

	endpoint := p.baseURL + "/api/v2/transaction/check"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: build transaction/check request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("va", p.va)
	httpReq.Header.Set("signature", sig)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: transaction/check http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: read transaction/check response body: %w", err)
	}

	var apiResp ipaymuTransactionCheckResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return StatusFailed, fmt.Errorf("ipaymu: parse transaction/check response (http=%d): %w", httpResp.StatusCode, err)
	}

	if apiResp.Status != 200 {
		return StatusFailed, fmt.Errorf("ipaymu transaction/check: status=%d %s", apiResp.Status, apiResp.Message)
	}

	return mapTransactionCheckStatus(ctx, apiResp.Data.Status, providerReference), nil
}

// mapTransactionCheckStatus maps an iPaymu transaction/check Data.Status int
// to our normalised PaymentStatus.
//
// iPaymu status codes (Go SDK constants):
//
//	 1 = success (Berhasil)
//	 0 = pending
//	-2 = expired
//	 2 = cancel
//	 4 = error
//	 5 = failed
func mapTransactionCheckStatus(ctx context.Context, code int, ref string) PaymentStatus {
	switch code {
	case 1:
		return StatusPaid
	case 0:
		return StatusPending
	case -2:
		return StatusExpired
	case 2, 4, 5:
		return StatusFailed
	default:
		slog.WarnContext(ctx, "ipaymu: unrecognised transaction/check status code — treating as failed",
			"status_code", code,
			"provider_reference", ref,
		)
		return StatusFailed
	}
}

// ---------------------------------------------------------------------------
// ListSettlements — STUB (Phase 6 manual ops, Phase 7 API)
// ---------------------------------------------------------------------------

// ListSettlements fetches the iPaymu daily settlement report.
//
// TODO(phase-7): implement using the iPaymu settlement API once the endpoint
// is documented for our account type. Settlement reconciliation in Phase 6 is
// a manual ops flow (platform-admin button triggers RunReconcile which calls
// this; stub means RunReconcile returns an error and ops must reconcile
// manually via the iPaymu dashboard).
func (p *IPaymuProvider) ListSettlements(_ context.Context, _ time.Time) ([]SettlementItem, error) {
	return nil, errIPaymuNotImplemented
}

// ---------------------------------------------------------------------------
// Signature helpers
// ---------------------------------------------------------------------------

// generateSignature produces the iPaymu outbound request signature.
//
// Algorithm (verified from github.com/ipaymu/ipaymu-go-api/signature.go):
//
//	bodyHash     = sha256(bodyBytes)
//	bodyHashHex  = hex.EncodeToString(bodyHash)   // lowercase
//	stringToSign = "POST:" + va + ":" + bodyHashHex + ":" + apiKey
//	signature    = hmac_sha256(apiKey, stringToSign) → hex (lowercase)
func generateSignature(bodyBytes []byte, va, apiKey string) (string, error) {
	// Step 1: SHA-256 of the raw body bytes.
	bodyHash := sha256.Sum256(bodyBytes)
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	// Step 2: construct the string-to-sign.
	stringToSign := "POST:" + va + ":" + bodyHashHex + ":" + apiKey

	// Step 3: HMAC-SHA256, keyed by apiKey.
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// computeWebhookSignature computes the expected inbound webhook HMAC.
//
// Algorithm (iPaymu WooCommerce plugin lines 642–700):
//  1. JSON-encode the map (encoding/json sorts map keys ascending —
//     matches PHP ksort + wp_json_encode for ASCII keys).
//  2. HMAC-SHA256 keyed by the VA number → hex.
//
// The inbound key is the VA (not the API key). Verified from WooCommerce plugin:
// `hash_hmac("sha256", $json_string, $va_number)`.
func computeWebhookSignature(data map[string]interface{}, va string) (string, error) {
	// encoding/json marshals map[string]interface{} with keys in ascending order,
	// which matches PHP's ksort behaviour for ASCII-only keys.
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal webhook map for signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(va))
	mac.Write(jsonBytes)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// ---------------------------------------------------------------------------
// Webhook parse helpers
// ---------------------------------------------------------------------------

// parseWebhookPayload decodes a webhook body that is either JSON or
// application/x-www-form-urlencoded. Returns both a raw map (for HMAC
// verification) and a typed struct (for field extraction).
func parseWebhookPayload(payload []byte, contentType string) (map[string]interface{}, ipaymuWebhookBody, error) {
	var rawMap map[string]interface{}
	var typed ipaymuWebhookBody

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Form-encoded: parse as URL query string.
		values, err := url.ParseQuery(string(payload))
		if err != nil {
			return nil, ipaymuWebhookBody{}, fmt.Errorf("parse form-encoded payload: %w", err)
		}
		rawMap = make(map[string]interface{}, len(values))
		for k, vs := range values {
			if len(vs) > 0 {
				rawMap[k] = vs[0]
			}
		}
		// Populate typed struct from the map.
		typed.TrxID = formVal(values, "trx_id")
		typed.ReferenceID = formVal(values, "reference_id")
		typed.Sid = formVal(values, "sid")
		typed.Status = formVal(values, "status")
		typed.StatusCode = formVal(values, "status_code")
		typed.Amount = formVal(values, "amount")
		typed.PaidOff = formVal(values, "paid_off")
		typed.PaymentNo = formVal(values, "payment_no")
		typed.PaymentMethod = formVal(values, "payment_method")
		typed.PaymentChannel = formVal(values, "payment_channel")
		typed.Signature = formVal(values, "signature")
	} else {
		// Default: JSON. Also handles content-type="" (missing header).
		if err := json.Unmarshal(payload, &typed); err != nil {
			return nil, ipaymuWebhookBody{}, fmt.Errorf("parse JSON payload: %w", err)
		}
		// Re-decode into generic map for signature computation.
		if err := json.Unmarshal(payload, &rawMap); err != nil {
			return nil, ipaymuWebhookBody{}, fmt.Errorf("parse JSON payload to map: %w", err)
		}
	}

	return rawMap, typed, nil
}

// formVal safely retrieves the first value for a form key.
func formVal(values url.Values, key string) string {
	if vs := values[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Status mapping
// ---------------------------------------------------------------------------

// mapWebhookStatus maps iPaymu's status_code (primary) and status string
// (fallback) to our normalised PaymentStatus.
//
// iPaymu status codes (Go SDK constants):
//
//	-2 = expired
//	 0 = pending
//	 1 = success (berhasil)
//	 2 = cancel
//	 5 = failed
func mapWebhookStatus(statusCode, statusStr string) PaymentStatus {
	code, err := strconv.Atoi(strings.TrimSpace(statusCode))
	if err == nil {
		switch code {
		case 1:
			return StatusPaid
		case 0:
			return StatusPending
		case -2:
			return StatusExpired
		default: // 2 (cancel), 5 (failed), others
			return StatusFailed
		}
	}

	// Fallback to status string when status_code is absent or unparseable.
	switch strings.ToLower(strings.TrimSpace(statusStr)) {
	case "berhasil":
		return StatusPaid
	case "pending":
		return StatusPending
	case "expired":
		return StatusExpired
	default:
		return StatusFailed
	}
}

// ---------------------------------------------------------------------------
// Misc helpers
// ---------------------------------------------------------------------------

// nonDigitRE matches any character that is not an ASCII digit.
var nonDigitRE = regexp.MustCompile(`[^0-9]`)

// sanitizePhone strips all non-digit characters from a phone number.
// iPaymu requires digits-only in the phone field.
func sanitizePhone(phone string) string {
	return nonDigitRE.ReplaceAllString(phone, "")
}

// parseAmountString converts a string amount (possibly with decimals) to int64 IDR.
// iPaymu may send "385000" or "385000.00".
func parseAmountString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Drop everything after a decimal point — IDR is integer-only.
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		s = s[:dot]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseIPaymuExpiry parses the iPaymu "Expired" timestamp string.
// iPaymu sends "2006-01-02 15:04:05" in Asia/Jakarta (WIB, UTC+7).
// Falls back to time.Now()+fallbackMinutes if parsing fails.
func parseIPaymuExpiry(ctx context.Context, expiredStr string, fallbackMinutes int) time.Time {
	if expiredStr == "" {
		slog.WarnContext(ctx, "ipaymu: empty Expired field in response, using fallback expiry",
			"fallback_minutes", fallbackMinutes)
		return time.Now().Add(time.Duration(fallbackMinutes) * time.Minute)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", expiredStr, jakartaLocation)
	if err != nil {
		slog.WarnContext(ctx, "ipaymu: failed to parse Expired field, using fallback expiry",
			"expired_str", expiredStr,
			"error", err,
			"fallback_minutes", fallbackMinutes,
		)
		return time.Now().Add(time.Duration(fallbackMinutes) * time.Minute)
	}
	return t
}

