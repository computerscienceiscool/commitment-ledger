# Recovery Checklist

Use this when local state is damaged or a checkout needs to be rebuilt from
backup.

## Minimal Flow

1. Restore `data/`, `records/`, `config/identities/`,
   `config/imported-identities/`, `docs/protocols/`, and `CHANGELOG.md`.
2. If you exported signer history separately, restore it with:

```bash
go run ./cmd/commitment-ledger identity restore --in /safe/path/identities.json
```

If that backup was created with `identity backup --include-imported-support`,
the same restore step also reinstalls imported signer and protocol support.

3. Run:

```bash
go run ./cmd/commitment-ledger doctor --repairable
```

4. Apply any machine-suggested repair flags, for example:

```bash
go run ./cmd/commitment-ledger repair --json --import-artifacts --import-support --identity-lineage
```

5. Re-run `doctor`, then spot-check with:

```bash
go run ./cmd/commitment-ledger status --exchange
go run ./cmd/commitment-ledger inspect COMMITMENT-...
go run ./cmd/commitment-ledger verify COMMITMENT-...
```

## Common Cases

### Missing imported artifact bytes

- Run `repair --import-artifacts`.

If `doctor` says the recorded bundle source path is gone, recover that original
bundle file or re-export it from another repo first.

### Missing imported signer or protocol support

- Run `repair --import-support`.

### Archived identity file exists under the wrong name

- Run `repair --identity-lineage`.

### Historical signer key is missing entirely

- Restore `config/identities/archive/` from backup.
- If you exported identity history separately, run `identity restore --in ...`.
- If that backup also carried imported support, it can restore that support at
  the same time.

### Warnings should fail CI

- Run `doctor --strict`.

## Limits

- `repair` does not recreate missing historical private keys from thin air.
- `repair` does not resolve conflicting imported state automatically.
- `identity restore` will refuse to overwrite a different local key file.
- `repair` cannot reconstruct imported artifact envelopes or support files once
  the recorded bundle source path is gone and no replacement bundle is
  available.
