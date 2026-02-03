-- Enable pgcrypto extension for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    dob DATE,
    sex TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index on email for faster lookups
CREATE INDEX idx_users_email ON users(email);
