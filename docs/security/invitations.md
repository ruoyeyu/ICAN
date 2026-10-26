# Invitation management

Administrators can view and copy new invitation codes in the existing invitation table, including after refresh or API restart. Creating a code no longer opens a one-time display dialog. Unused or expired invitations can be revoked: the row is deleted, and the code immediately stops registering accounts. Used invitations remain as usage records without a revoke action; the backend also rejects their deletion. Registration and deletion use conditional database writes so a concurrent request cannot both consume and delete the same code.

## Storage and permissions

Registration continues to use the unique SHA-256 `code_hash`. New codes additionally store `encrypted_code` using the existing AES-256-GCM implementation and original `DATA_ENCRYPTION_KEY`, with associated data bound to the invitation ID. Plaintext is returned only through administrator invitation endpoints with `Cache-Control: no-store`; neither hashes nor ciphertext are serialized. No new dependency, cryptographic scheme or credential is introduced.

Legacy codes stored only as hashes cannot be recovered for display. They appear as legacy entries, remain valid under their existing expiration/use rules and can still be revoked while unused. The upgrade does not rotate or replace them. Decryption failures are reported rather than treated as missing legacy values.

## Upgrade and rollback

`repository.migrateInvitationsV1` explicitly creates the invitation table for new installations or adds only `encrypted_code` to an existing table. Repeated execution is supported. In line with the new deletion behavior, it removes legacy rows only where `revoked = true AND used_by = ''`; every used record is preserved, including those an older backend had marked revoked. Existing hashes, IDs, expiration dates, account links and all other application data remain unchanged.

Take the normal consistent database/media/configuration backup before updating, retaining the original encryption key. A backup-copy fixture verifies the old schema, the additive migration twice, scoped cleanup and old-model reads. The previous image can read the existing table with the extra column present; it ignores ciphertext and reverts to its old invitation UI. Deleted records are not recreated when only changing images. Restoring an earlier database snapshot may restore old invitations and must account for that snapshot's use/revocation state.

## Verification

Free integration tests cover encrypted storage, repeated administrator reads, no-store headers, anonymous/user denial, irreversible deletion from the active table, rejected use after deletion, used-record retention, legacy-code registration, swapped ciphertext rejection and concurrent consume/delete requests. The isolated production browser fixture covers visible codes, clipboard copying, refresh, API restart with the same database/key, deletion and used-record behavior. No deployment server or paid model API is involved.

Local results are recorded in `evidence/invitations-local.json`. [CI 40347559560](https://github.com/ruoyeyu/ICAN/actions/runs/40347559560) passed; the [publication evidence](evidence/invitations-publication.json) records the immutable image and anonymous manifest/config verification. See the [pull-only deployment guide](ghcr-deployment.zh-CN.md).
