package billing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type testRow func(...any) error

func (r testRow) Scan(dst ...any) error { return r(dst...) }

type emptyRows struct{ pgx.Rows }

func (emptyRows) Close()            {}
func (emptyRows) Err() error        { return nil }
func (emptyRows) Next() bool        { return false }
func (emptyRows) Scan(...any) error { return nil }

type tokenOnlyTx struct {
	pgx.Tx
	walletTouched bool
}

func (tx *tokenOnlyTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "user_token_package") {
		return emptyRows{}, nil
	}
	return nil, errors.New("unexpected query")
}

func (tx *tokenOnlyTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "wallet_balance") {
		tx.walletTouched = true
		return testRow(func(dst ...any) error { *dst[0].(*float64) = 100000; return nil })
	}
	if strings.Contains(sql, "INSERT INTO usage_log") {
		return testRow(func(dst ...any) error { *dst[0].(*string) = "usage-id"; return nil })
	}
	return testRow(func(...any) error { return errors.New("unexpected query row") })
}

func (tx *tokenOnlyTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "wallet_balance") || strings.Contains(sql, "credit_transaction") {
		tx.walletTouched = true
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestChargeTxMenolakTanpaTokenDanTidakMenyentuhWallet(t *testing.T) {
	tx := &tokenOnlyTx{}
	_, err := ChargeTx(context.Background(), tx, "user-id", "", "model", 10, 10, 0)
	if !errors.Is(err, ErrInsufficientCredit) {
		t.Fatalf("err=%v, mau ErrInsufficientCredit", err)
	}
	if tx.walletTouched {
		t.Fatal("wallet tidak boleh dibaca atau diubah untuk penggunaan AI")
	}
}
