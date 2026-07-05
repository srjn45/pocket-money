-- Zero-interest loans repaid via monthly EMI debits (§5.3).
CREATE TYPE loan_status AS ENUM ('requested', 'active', 'rejected', 'closed');

CREATE TABLE loans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id      UUID   NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id       UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,  -- borrower
    principal     BIGINT NOT NULL CHECK (principal > 0),        -- minor units
    installments  INT    NOT NULL CHECK (installments > 0),
    emi_amount    BIGINT NOT NULL CHECK (emi_amount > 0),       -- ceil(principal/installments)
    start_period  CHAR(7),                                      -- 'YYYY-MM'; NULL until active
    status        loan_status NOT NULL DEFAULT 'requested',
    note          TEXT,
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_at    TIMESTAMPTZ
);

-- Engine reads active loans per group; handlers list per group (+ optional user/status).
CREATE INDEX idx_loans_group_status ON loans (group_id, status);
CREATE INDEX idx_loans_group_user   ON loans (group_id, user_id);

-- Add the FK deferred since migration 009 (WP-1.2 §1.0): ledger_entries.loan_id -> loans.
-- ON DELETE SET NULL: deleting a loan orphans its emi rows' loan_id rather than
-- cascade-deleting money history (the ledger is immutable audit; §5.1).
ALTER TABLE ledger_entries
    ADD CONSTRAINT fk_ledger_entries_loan
    FOREIGN KEY (loan_id) REFERENCES loans(id) ON DELETE SET NULL;
