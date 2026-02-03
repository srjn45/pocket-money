-- Create ledger status enum
CREATE TYPE ledger_status AS ENUM ('approved', 'pending_approval', 'rejected');

-- Create ledger_entries table
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chore_id UUID NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    amount DECIMAL(12, 2) NOT NULL,
    status ledger_status NOT NULL,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    rejected_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_ledger_entries_group_id ON ledger_entries(group_id);
CREATE INDEX idx_ledger_entries_group_status ON ledger_entries(group_id, status);
CREATE INDEX idx_ledger_entries_user_id ON ledger_entries(user_id);
