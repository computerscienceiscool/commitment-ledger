# Trust And Verification

## Purpose

This file explains what the local `verify` flow proves today, and what it
does not prove.

For the machine-readable output contract of `verify --json`, use
`docs/machine-readable-contracts.md` as the versioned reference.

## What `verify` Checks

When you run:

```bash
go run ./cmd/commitment-ledger verify COMMITMENT-...
```

the `verify` flow resolves the referenced artifact and checks:

1. the raw artifact bytes exist in local CAS
2. the grid envelope decodes successfully
3. the envelope's protocol selector, payload bytes, and proof bytes produce the
   same CIDs recorded in `data/artifacts.jsonl`
4. the proof signature verifies over the carried protocol selector and payload
5. the proof signer name and key ID match the local identity file
6. the proof public key matches the public key stored in that local identity
   file
7. the artifact's `protocol_pcid` matches a locally frozen protocol doc, if the
   repo has that doc loaded
8. the output shows whether the signer and protocol support came from built-in
   local state or imported support
9. the output includes the latest recorded import provenance when the artifact
   entered the repo through `import` or `receive`

## What `verify` Does Not Check

`verify` does not prove:

- that the signer was socially trustworthy
- that the signer was accepted by some upstream PromiseGrid trust source
- that the payload claims were true in the real world
- that another peer independently agrees with the assessment
- that the local identity store itself was never tampered with

In other words, `verify` proves local structural and cryptographic consistency.
It does not magically solve governance, reputation, or shared-trust questions.

## Current Trust Model

Today, Commitment Ledger is local-first:

- artifacts live in local CAS
- identity material lives under `config/identities/`
- imported public signer material can live under `config/imported-identities/`
- protocol docs live under `docs/protocols/`
- imported protocol docs can live under `data/imported-protocols/`
- import and receive provenance lives under `data/imports.jsonl`
- optional trust policy lives under `config/trust-policy.json`
- conformance claims are local statements by this implementation

That means verification is strongest when you are checking artifacts emitted by
this same local repo state and signer store. Imported bundles can extend what
the repo can verify locally, but they do not by themselves create shared trust.
The repo can now at least tell you where imported support came from inside this
local ledger state; it still cannot decide whether that source deserves trust.

## Trust Policy

When `config/trust-policy.json` exists, the repo applies a local policy layer
on top of cryptographic verification.

Current fields:

- `trust_built_in_signers`
- `trust_built_in_protocols`
- `trusted_signers`
- `trusted_protocol_pcids`
- `trusted_import_modes`
- `trusted_import_path_prefixes`

That policy does not change artifact bytes or protocol meaning. It only changes
how this local operator classifies signer, protocol, and import-source trust.

## Practical Reading

If `verify` succeeds, the useful interpretation is:

- these bytes are internally consistent
- the signature is valid for the carried payload and protocol selector
- the signer matches the local identity material you currently have
- the artifact can be tied back to a known local protocol doc when one is
  present
- the repo can show whether that support was built-in or imported and when the
  artifact was imported into this local ledger state
- if a trust policy exists, the repo can also say whether this local policy
  treats the signer, protocol, and import source as trusted

If `verify` fails because of signer mismatch, missing signer identity, or CID
mismatch, treat that as a real integrity problem until explained.
