package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// txKey is the unexported context key used to stash the in-flight *gorm.DB
// transaction so repositories within a WithTx call operate on the same tx.
type txKey struct{}

// TxManager implements service.TxManager using *gorm.DB transactions.
type TxManager struct {
	db *gorm.DB
}

// NewTxManager constructs a TxManager.
func NewTxManager(db *gorm.DB) *TxManager { return &TxManager{db: db} }

// WithTx begins a GORM transaction, injects it into the context, and calls fn.
// On fn error the transaction is rolled back; otherwise it is committed.
// Used by services that need to span multiple repository writes atomically.
func (tm *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// If there is ALREADY a tx in the context (e.g., the per-request tx set by
	// the tenant middleware), reuse it — no nested transactions.
	if existing, ok := ctx.Value(txKey{}).(*gorm.DB); ok && existing != nil {
		return fn(ctx)
	}
	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		if err := fn(txCtx); err != nil {
			return fmt.Errorf("tx: %w", err)
		}
		return nil
	})
}

// BeginForRequest opens a new transaction and returns (a) a derived context
// carrying that tx (so repository.dbFromContext picks it up) and (b) a finisher
// closure the caller invokes at end-of-request to commit or roll back.
//
// The finisher swallows "already committed/rolled-back" errors so double-calling
// it is safe.
//
// Intended sole caller: the tenant Gin middleware. Regular services should
// use WithTx for scoped-write atomicity.
func (tm *TxManager) BeginForRequest(ctx context.Context) (context.Context, RequestTxFinisher, error) {
	tx := tm.db.WithContext(ctx).Begin()
	if err := tx.Error; err != nil {
		return ctx, noopFinisher{}, fmt.Errorf("begin request tx: %w", err)
	}
	return context.WithValue(ctx, txKey{}, tx), &gormRequestTx{tx: tx}, nil
}

// SetTenantContext sets the two GUC variables that the PostgreSQL RLS policies
// check ('app.current_tenant', 'app.current_user'). The is_local flag is true,
// so the setting lives for the duration of the current transaction only — any
// leak into a pooled connection is impossible.
//
// MUST be called inside a transaction (usually the one started by
// BeginForRequest, or inside a WithTx closure). When there is no tx in ctx
// this call still executes on the pooled connection but the setting is
// immediately dropped on commit — the caller is expected to have opened a tx
// beforehand.
func (tm *TxManager) SetTenantContext(ctx context.Context, tenantID, userID string) error {
	db := dbFromContext(ctx, tm.db)
	return db.Exec(
		"SELECT set_config('app.current_tenant', ?, true), set_config('app.current_user', ?, true)",
		tenantID, userID,
	).Error
}

// RequestTxFinisher is returned by BeginForRequest so middleware can decide
// whether to commit or roll back once the downstream handlers are done.
type RequestTxFinisher interface {
	Commit() error
	Rollback() error
}

type gormRequestTx struct {
	tx   *gorm.DB
	done bool
}

func (g *gormRequestTx) Commit() error {
	if g.done {
		return nil
	}
	g.done = true
	if err := g.tx.Commit().Error; err != nil {
		// Already-closed / already-rolled-back errors are fine — they mean
		// the tx was already finalised, not a data problem.
		if errors.Is(err, gorm.ErrInvalidTransaction) {
			return nil
		}
		return err
	}
	return nil
}

func (g *gormRequestTx) Rollback() error {
	if g.done {
		return nil
	}
	g.done = true
	if err := g.tx.Rollback().Error; err != nil {
		if errors.Is(err, gorm.ErrInvalidTransaction) {
			return nil
		}
		return err
	}
	return nil
}

type noopFinisher struct{}

func (noopFinisher) Commit() error   { return nil }
func (noopFinisher) Rollback() error { return nil }

// dbFromContext extracts the transaction-scoped *gorm.DB from context if one
// was injected by WithTx or BeginForRequest, falling back to the root DB. All
// repository methods must use this helper so they participate in the caller's
// transaction.
func dbFromContext(ctx context.Context, root *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return root.WithContext(ctx)
}
