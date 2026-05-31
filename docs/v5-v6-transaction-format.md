# v5 vs v6 Transaction Format Comparison

## Feature Flag Dependency

All v6 read/write paths are gated behind `#[cfg(zcash_unstable = "nu7")]` in `librustzcash`.

| Component | Feature flag | zkool2 enabled? |
|-----------|-------------|-----------------|
| `TxVersion::V6` | `nu7` | ✅ (via `pczt` transitive) |
| `read_v6` / `write_v6` | `nu7` | ✅ |
| `zip233_amount` field | `nu7` + `zip-233` | ✅ |
| `issue_bundle` (read/write) | `nu7` | ✅ |
| OrchardZSA flavor | `nu7` | ✅ |

**Key**: `pczt` crate (v0.6) enables `nu7` transitively. zkool2 Cargo.toml explicitly enables `zip-233`.

---

## Transaction Header

| Field | v5 (bytes) | v6 (bytes) | Notes |
|-------|-----------|-----------|-------|
| version | 4 | 4 | v5=`0x05000080`, v6=`0x06000080` |
| version_group_id | 4 | 4 | v5=`0x26A7270A`, v6=`0x77777777` |
| consensus_branch_id | 4 | 4 | |
| lock_time | 4 | 4 | |
| expiry_height | 4 | 4 | |
| zip233_amount | — | 8 | **Only if `#[cfg(feature = "zip-233")]`** |
| **Header total** | **20** | **28** | |

### ⚠️ Go parser issue (minor)
Go `parseV6` unconditionally skips 8 bytes for `zip233_amount`. This is correct when builder has `zip-233` enabled, but would be an 8-byte offset error if a v6 transaction were ever serialized without that feature. Currently not triggered because `pczt` always enables it.

---

## Transparent Section

**Identical** in v5 and v6, except v6 adds sighash_info after vouts:

| Field | v5 | v6 |
|-------|----|----|
| vin / vout | ✓ | ✓ |
| sighash_info (per vin) | — | CompactSize + data |

---

## Sapling Bundle

### Spend description (per-spend, before proofs/sigs)

| Field | Size | v5 | v6 |
|-------|------|----|----|
| cv | 32 | ✓ | ✓ |
| nullifier | 32 | ✓ | ✓ |
| rk | 32 | ✓ | ✓ |
| **Per-spend subtotal** | **96** | | |

### Output description (per-output, before proofs)

| Field | Size | v5 | v6 |
|-------|------|----|----|
| cv | 32 | ✓ | ✓ |
| cmu | 32 | ✓ | ✓ |
| ephemeralKey | 32 | ✓ | ✓ |
| encCiphertext | 580 | ✓ | ✓ |
| outCiphertext | 80 | ✓ | ✓ |
| **Per-output subtotal** | **756** | | |

### Collective fields (after all spends/outputs)

| Field | Size per element | v5 | v6 |
|-------|-----------------|----|----|
| valueBalance | 8 | ✓ | ✓ |
| anchor | 32 (if spends>0) | ✓ | ✓ |
| spend proofs | 192 each | ✓ | ✓ |
| **spend auth sigs** | — | **raw 64 bytes** | **versioned sig** |
| output proofs | 192 each | ✓ | ✓ |
| **binding sig** | — | **raw 64 bytes** | **versioned sig** |

### 🔴 BUG 1: Go `parseV5` uses `skipVersionedSignature` for sapling sigs

Rust v5 sapling writes **raw 64-byte** spend auth sigs and binding sig. Go `parseV5` uses `skipVersionedSignature` which reads `CompactSize + sighash + 64`. This causes an **overread of ~6 bytes per sig** for v5 transactions.

However, parseV6 correctly uses versioned sigs for sapling. This bug only affects v5, not v6.

---

## Orchard Bundle

### Action description (per-action, before auth)

| Field | v5 (OrchardVanilla) | v6 (OrchardZSA) |
|-------|---------------------|-----------------|
| cv | 32 | 32 |
| nullifier | 32 | 32 |
| rk | 32 | 32 |
| cmx | 32 | 32 |
| ephemeralKey | 32 | 32 |
| encCiphertext | 580 | **612** |
| outCiphertext | 80 | 80 |
| **Per-action subtotal** | **820** | **852** |

### Bundle structure after actions

| Field | v5 | v6 |
|-------|----|----|
| num_action_groups (CompactSize) | — | **✓** |
| nActions (CompactSize) | ✓ (implicit from Vector) | ✓ |
| flags | 1 byte (before valueBalance) | 1 byte |
| valueBalance | 8 bytes (before anchor) | 8 bytes (after spendAuthSigs) |
| anchor | 32 bytes (after valueBalance) | 32 bytes (before n_ag_expiry) |
| n_ag_expiry_height | — | **4 bytes (must be 0)** |
| burn | — | **CompactSize + 40 per entry** |
| proofs | CompactSize + data | CompactSize + data |
| spend auth sigs | **raw 64 bytes each** | **versioned sig each** |
| binding sig | **raw 64 bytes** | **versioned sig** |

### Key structural differences (v5 → v6)

1. **num_action_groups prefix**: v6 adds a CompactSize before actions. 0 means no orchard bundle.
2. **OrchardZSA flavor**: encCiphertext grows from 580 → 612 bytes (+32 for asset field).
3. **Field reordering**: v5: flags → valueBalance → anchor. v6: flags → anchor → n_ag_expiry → burn → proofs → sigs → valueBalance → bindingSig.
4. **n_ag_expiry_height**: New 4-byte field in v6 (always 0 for NU7).
5. **burn**: New field in v6 (vector of AssetBase + NoteValue).
6. **Versioned signatures**: v5 uses raw 64-byte signatures; v6 uses CompactSize(sighash_type) + sighash + 64-byte sig.

### 🔴 BUG 2: Go `parseV6` reads CompactSize count for orchard spend auth sigs

Rust `write_v6_bundle` uses `Array::write` which writes exactly N versioned sigs **without** a length prefix (the count is implicit from nActions). Go `parseV6` reads a `CompactSize` count first, then reads that many sigs. This causes the parser to interpret the first byte of the first versioned sig's sighash CompactSize as the sig count, shifting all subsequent parsing.

---

## Issue Bundle (v6 only)

| Field | Format |
|-------|--------|
| issuer_key | Vector\<u8\> (CompactSize + data) |
| n_actions | CompactSize |
| actions | (if issuer non-empty) see below |
| authorization | (if issuer non-empty) sighash_info + signature |

**Empty issue bundle (no issuance)**: `[0x00, 0x00]` — issuer_len=0, nActions=0. Always present in v6.

### 🔴 BUG 3: Go `parseV6` unconditionally parses orchard bundle then issue bundle

If the orchard bundle parsing consumes wrong byte counts (due to BUG 2 or other offset errors), the parser reaches `skipIssueBundle` at the wrong position, reading garbage as `issuerLen`. This causes the observed error: `"could not skip issue bundle issuer key"`.

---

## Versioned Signature Format (v6 only)

| Field | Size |
|-------|------|
| sighash_info | Vector\<u8\> = CompactSize(len) + len bytes |
| signature | 64 bytes |

Go `skipVersionedSignature` reads: `CompactSize(sighashLen) + sighashLen bytes + 64 bytes`. This matches Rust.

For v5, Rust writes raw 64-byte sigs (no sighash info). Go incorrectly uses `skipVersionedSignature` in `parseV5` (see BUG 1).

---

## Go Parser Function Dispatch

```
ParseFromSlice
  ├── parsePreV5  (version 1-4)
  ├── parseV5     (version 5)
  └── parseV6     (version 6, ZIP-230, version_group=0x77777777)
```

---

## Summary of Bugs

| # | File | Function | Bug | Impact |
|---|------|----------|-----|--------|
| 1 | transaction.go:582,590 | `parseV5` | Uses `skipVersionedSignature` for sapling sigs; should be raw 64-byte | v5 txs with sapling overread by ~6 bytes/sig |
| 2 | transaction.go:797-798 | `parseV6` | Reads CompactSize `sigCount` for orchard spendAuthSigs; Rust writes Array (no prefix) | Parses wrong number of sigs, all subsequent fields shifted |
| 3 | transaction.go:832-836 | `parseV6` | Unconditional `skipIssueBundle` call — fails if orchard parsing is offset | Masked by BUG 2; crashes with "could not skip issue bundle issuer key" |

### Root cause of the current sync failure

**BUG 2** causes the orchard spend auth sigs to be parsed incorrectly, shifting the parser position. This causes `skipIssueBundle` to read from the wrong offset, interpreting non-zero bytes as `issuerLen`, which then fails `Skip(issuerLen)` because insufficient bytes remain.

---

## Verification Checklist

| Layer | v6 Writer | v6 Parser | Aligned? |
|-------|-----------|-----------|----------|
| zkool2 (Rust builder) | `write_v6` via `pczt` | — | — |
| zebrad (node) | Rust `zcash_primitives` | Rust `zcash_primitives` | ✅ same crate |
| lightwalletd (Go) | — | `parseV6` | ❌ BUGS 2, 3 |
