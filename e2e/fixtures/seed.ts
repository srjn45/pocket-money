// Canonical seed credentials — mirrors backend/cmd/seed/main.go constants.
// IDs, balances, and periods are in e2e/fixtures/seed.summary.json (generated
// by `make seed` at CI time, git-ignored).

export const SEED = {
  DEMO_PASSWORD: 'demo1234',
  GROUP_NAME: 'Sharma Family',
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

export type SeedSummary = {
  current_period: string;
  users: {
    head: { id: string; email: string; name: string };
    aarav: { id: string; email: string; name: string };
    diya: { id: string; email: string; name: string };
  };
  group: { id: string; name: string };
  expected_balances: Record<string, number>;
  loan_outstanding: number;
};
