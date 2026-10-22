package handler

import (
	"net/url"
	"testing"
)

func TestDokuSignatureUsesCanonicalNonSNAPHeaders(t *testing.T) {
	body := []byte(`{"order":{"amount":20000,"invoice_number":"N2M0123456789ABCDEF012345"}}`)
	got := dokuSignature(
		"MCH-TEST",
		"00000000-0000-4000-8000-000000000001",
		"2026-10-08T00:00:00Z",
		"/checkout/v1/payment",
		body,
		"test-secret",
	)
	const want = "HMACSHA256=xn98tw2ROvt1IRvIZlHdoQfbB+kvCi5WoCeOP04g7KI="
	if got != want {
		t.Fatalf("dokuSignature() = %q, want %q", got, want)
	}
}

func TestDokuGetSignatureOmitsDigest(t *testing.T) {
	got := dokuSignature(
		"MCH-TEST",
		"00000000-0000-4000-8000-000000000002",
		"2026-10-08T00:00:00Z",
		"/orders/v1/status/N2M0123456789ABCDEF012345",
		nil,
		"test-secret",
	)
	const want = "HMACSHA256=hXqxTy2hLD6x8omQg3dQDfbBqdeFzBNTtoF/OTFmOvE="
	if got != want {
		t.Fatalf("dokuSignature(GET) = %q, want %q", got, want)
	}
}

func TestDokuInvoiceIsCardSafeAndDeterministic(t *testing.T) {
	got := dokuInvoiceNumber("01234567-89ab-cdef-0123-456789abcdef")
	const want = "N2M0123456789abcdef01234567"
	if got != want {
		t.Fatalf("dokuInvoiceNumber() = %q, want %q", got, want)
	}
	if len(got) > 30 || !validDokuInvoiceNumber(got) {
		t.Fatalf("invoice must stay card-safe: %q", got)
	}
}

func TestDokuPaymentStatusMapsOnlyVerifiedFinalStates(t *testing.T) {
	cases := []struct {
		order, transaction, want string
	}{
		{"ORDER_GENERATED", "SUCCESS", "PAID"},
		{"ORDER_EXPIRED", "PENDING", "EXPIRED"},
		{"ORDER_GENERATED", "FAILED", "FAILED"},
		{"ORDER_GENERATED", "PENDING", "PENDING"},
		{"ORDER_GENERATED", "REDIRECT", "PENDING"},
	}
	for _, tc := range cases {
		if got := dokuPaymentStatus(tc.order, tc.transaction); got != tc.want {
			t.Fatalf("dokuPaymentStatus(%q, %q) = %q, want %q", tc.order, tc.transaction, got, tc.want)
		}
	}
}

func TestDokuAmountDoesNotAcceptUnderpayment(t *testing.T) {
	if !dokuAmountAccepted(10000, 10282) {
		t.Fatal("provider fee total rejected")
	}
	if dokuAmountAccepted(10000, 9999) {
		t.Fatal("underpayment accepted")
	}
}

func TestDokuFinishURLKeepsOnlyPaymentContext(t *testing.T) {
	raw := dokuFinishURL("https://www.node2mint.com", "/payment?kind=package&id=pkg-1&amount=20000&doku_invoice=attacker&other=value", "N2M0123456789abcdef01234567")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "www.node2mint.com" || u.Path != "/payment" {
		t.Fatalf("unsafe finish URL: %q", raw)
	}
	q := u.Query()
	if q.Get("doku") != "finish" || q.Get("doku_invoice") != "N2M0123456789abcdef01234567" || q.Get("kind") != "package" || q.Get("id") != "pkg-1" || q.Get("amount") != "20000" {
		t.Fatalf("payment context lost: %q", raw)
	}
	if q.Get("other") != "" || len(q["doku_invoice"]) != 1 {
		t.Fatalf("untrusted redirect query survived: %q", raw)
	}
}

func TestDokuPendingPayloadExposesCreatingWithoutCheckoutURL(t *testing.T) {
	payload := dokuPendingPayload("N2M0123456789abcdef01234567", "CREATING", "", "sandbox")
	if payload["creating"] != true || payload["checkout_url"] != nil || payload["status"] != "CREATING" {
		t.Fatalf("creation state must remain visible without a redirect URL: %#v", payload)
	}
}

func TestDokuCreationFailureRequiresDefinitiveProviderRejection(t *testing.T) {
	if !dokuCreateRejected(&dokuHTTPError{operation: "create payment", status: 400}) {
		t.Fatal("4xx rejection must close a creation intent")
	}
	for _, status := range []int{408, 409, 429, 500} {
		if dokuCreateRejected(&dokuHTTPError{operation: "create payment", status: status}) {
			t.Fatalf("ambiguous HTTP %d must remain reconcilable", status)
		}
	}
	if !dokuStatusNotFound(&dokuHTTPError{operation: "check status", status: 404}) {
		t.Fatal("only a status 404 may age out CREATING")
	}
}
