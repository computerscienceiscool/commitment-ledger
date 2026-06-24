# CHANGELOG

## Unreleased

### Conformance

These entries are the repo-level publication surface for implementation
conformance claims. Until upstream PromiseGrid freezes one shared Commitment
Ledger app contract, these entries name the exact local frozen spec doc-CIDs
this implementation claims to speak. The signed `conformance` artifact is the
machine-readable companion claim.

```changelog-entry
claim:           implements
spec:            bafkreibbmx4swcujke52q5ak7bx2p5so6caupcxh7trb25owz5bxmj5dnq
scope:           full
breaking-change: false
notes:           Emits new commitment artifacts against local frozen `commitment-promise-v1`.
```

```changelog-entry
claim:           implements
spec:            bafkreiekzyj4clq3meomv6ijoef6fyjimtxhfljgapickvzzxbbe3g2mie
scope:           full
breaking-change: false
notes:           Emits new evidence artifacts against local frozen `commitment-evidence-v2`.
```

```changelog-entry
claim:           implements
spec:            bafkreiewlmgvkcp7pu6yrr7tw63zpvuqg6jib3b2xpgoeqihlu5wfuvomi
scope:           full
breaking-change: false
notes:           Emits new assessment artifacts against local frozen `commitment-assessment-v2`.
```

```changelog-entry
claim:           implements
spec:            bafkreigflnongawbrzafw6o2uygawi2npqz5uskr2n7av3lo5o5z4fg2zy
scope:           full
breaking-change: false
notes:           Emits signed local implementation conformance claims against `implementation-conformance-v1`.
```

```changelog-entry
claim:           partially-implements
spec:            bafkreigbk35utys33bfx7cz3orxzpidg54earaqhlg6axhdbfpuz7p7n4u
scope:           historical-read-only
breaking-change: false
notes:           Retained for reading older local evidence artifacts; not emitted by current commands.
```

```changelog-entry
claim:           partially-implements
spec:            bafkreiao2fhgjph3rwsszk65hzzj4tr55m4vsawkinvmigdurjr4btntdi
scope:           historical-read-only
breaking-change: false
notes:           Retained for reading older local assessment artifacts; not emitted by current commands.
```
