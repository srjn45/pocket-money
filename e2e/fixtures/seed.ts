// Canonical seed credentials — mirrors backend/cmd/seed/main.go constants.
// IDs, balances, and periods are in e2e/fixtures/seed.summary.json (generated
// by `make seed` at CI time, git-ignored).

export const SEED = {
  DEMO_PASSWORD: 'demo1234',
  GROUP_NAME: 'Sharma Family',
  // Second, EUR-currency group (V3-1.1 §5): head = Priya, member = Aarav.
  GROUP2_NAME: 'Sharma Europe Trip',
  HEAD: {
    email: 'head@demo.test',
    name: 'Priya Sharma',
  },
  AARAV: {
    email: 'aarav@demo.test',
    name: 'Aarav Sharma',
  },
  DIYA: {
    email: 'diya@demo.test',
    name: 'Diya Sharma',
  },
} as const;

/** ISO-4217 code — matches backend Money.currency / groups.currency (V3-1.1). */
export type Currency = 'EUR' | 'USD' | 'INR';

export type SeedSummary = {
  current_period: string;
  users: {
    head: { id: string; email: string; name: string };
    aarav: { id: string; email: string; name: string };
    diya: { id: string; email: string; name: string };
  };
  group: { id: string; name: string; currency: Currency };
  // Second group in a different currency (EUR) — proves per-group currency (D7).
  group2: { id: string; name: string; currency: Currency };
  // The seeded EUR "Gelato treat" chore amount in minor units (€12.50 = 1250).
  group2_amount: number;
  expected_balances: Record<string, number>;
  loan_outstanding: number;
};
