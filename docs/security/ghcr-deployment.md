# Pull-only Docker deployment

ICAN 0.1.0 is built on GitHub Actions for **linux/amd64**. The server needs Docker Engine, Compose v2 and Python 3 for the read-only policy checker; it does not compile Go, Node or Director code. No workflow connects to a deployment server.

## Publication contract

`main` pushes affecting code or the workflow, and manual runs on `main`, run the existing Go/frontend/Director tests and builds, dependency audits, container/Compose smoke tests, runtime regressions, native inventory and image vulnerability scan. HIGH/CRITICAL image matches block publication; all severities remain in the raw report. A failed check, scanner error or missing artifact prevents publication. Pull requests only check; they cannot publish.

A separate job with `packages: write` loads the exact checked Docker archive, verifies its SHA256, image ID and source revision label, and pushes it with the ephemeral `GITHUB_TOKEN`. No PAT, application Key or server credential is needed in the repository, build arguments or image. Registry authentication happens only after checks. The token is supplied through stdin and removed after the job.

Find the successful workflow's Summary and download `published-image/publication.json`. It records the full source commit, image ID, GHCR digest, platform and check-run link. `image-security-evidence` contains the full scan, SBOM, native inventory and Compose policy report. The transport archive expires after seven days; retain required evidence separately. Artifact names appear as downloads in the run page, not as repository directories.

Tags have the form `sha-<full-commit>-run-<run-id>-<attempt>`. Tags are locators, **not immutable references**. Deploy only the full `ghcr.io/ruoyeyu/ican@sha256:…` from the publication manifest. There is no automatically deployed `latest` tag. Keep previous digests available for rollback; do not delete GHCR versions used by live deployments or backups.

If GHCR rejects the first push, the workflow fails. An owner must check that Actions can create/write the repository-linked package; an existing package must grant this repository Actions access. Do not bypass checks or use a broad personal token to hide a permission failure. Public source does not guarantee a public package: anonymous pull requires the package to be public. For a private package, authenticate the server with a read-only package credential via `docker login --password-stdin`; keep it outside the app environment.

## First staging start

Resource limits are fixed in Compose: **1.6 CPUs, 1536 MiB (1.5 GiB), no extra swap**. Memory exhaustion can terminate the app; these limits do not replace the outstanding upload/concurrency/disk-quota work. PID/tmpfs limits, authentication rate and supplier-link lifetime still require explicit operator values; no new production defaults are invented here.

Obtain `docker-compose.yml`, `.env.example` and `scripts/check-container.py` from the exact source commit recorded in the manifest. Use a separate deployment folder and a pre-created, isolated data directory writable by container UID/GID 10001 (verify host UID mapping for Rootless Docker). Never recursively change ownership of an existing shared directory.

Create an operator-owned environment file outside the checkout with mode 0600. Fill the example's variables, including:

- `ICAN_IMAGE`: the full digest reference from `publication.json`.
- `ICAN_DATA_DIR`: the absolute persistent data directory; Compose refuses to create a missing directory.
- `ICAN_HOST_PORT`: a free local port, published on 127.0.0.1 only.
- `PUBLIC_ORIGIN`, administrator bootstrap credentials, independent `JWT_SECRET` and `DATA_ENCRYPTION_KEY`, and the remaining explicit limits.

Use unique initial secrets from your operator's secret-management process. On upgrades **reuse the original secrets and data directory**. Do not run bootstrap secret generation again. Never commit the environment file, print `docker compose config` without `--quiet`, or share full `docker inspect` environment output.

Set shell variables to your reviewed deployment values (these are paths and a public image digest, not secrets):

```sh
ICAN_OPERATOR_ENV=/secure/operator/ican.env
ICAN_VERIFIED_IMAGE='ghcr.io/ruoyeyu/ican@sha256:<digest-from-publication.json>'
ICAN_REVIEWED_DATA_DIR=/srv/ican/data
ICAN_REVIEWED_HOST_PORT=3001

docker pull "$ICAN_VERIFIED_IMAGE"
ICAN_REVIEWED_IMAGE_ID=$(docker image inspect "$ICAN_VERIFIED_IMAGE" --format '{{.Id}}')
# Compare this ID and the revision label with publication.json before continuing.
docker image inspect "$ICAN_VERIFIED_IMAGE" --format '{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}'

docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml config --quiet
docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml create --no-build app
ICAN_CONTAINER_ID=$(docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml ps -aq app)
python3 scripts/check-container.py "$ICAN_CONTAINER_ID" \
  --image-id "$ICAN_REVIEWED_IMAGE_ID" \
  --data-dir "$ICAN_REVIEWED_DATA_DIR" --host-port "$ICAN_REVIEWED_HOST_PORT"
# Only start after the checker succeeds and the host's isolation has been reviewed.
docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml start app
curl --fail "http://127.0.0.1:$ICAN_REVIEWED_HOST_PORT/api/health"
```

Compose keeps `pull_policy: never`: pull the chosen digest explicitly before creating the container. It has no `build` service. Avoid `--pull always`, moving tags and unattended update agents. A health response is only a basic check; verify login, media, configuration decryption and task recovery too.

## Updating without losing data

The bind mount `/app/data` contains SQLite `ican.db` (and WAL/SHM), media and logs. User/configuration records, API addresses/Keys, channels, canvas and task records remain in the database. Unsynced browser-only data is not included. New images do not contain or replace these records.

1. Download the new publication manifest and check results. Pull the new digest before stopping the existing app. Keep the old digest, source commit, Compose file, environment file and original encryption/signing keys.
2. Validate the new version against a backup copy first. Current `AutoMigrate` is not a complete versioned migration or downgrade mechanism; real cross-version recovery remains a release gate.
3. Schedule a maintenance window and stop app writes. With the app stopped, back up the **entire data directory** (including any WAL/SHM), preserve permissions, and independently back up the operator environment/keys. Store backups outside app-writable paths. Do not copy only a live SQLite `.db` file.
4. Verify backup readability/checksums and a restore on an isolated copy. Change only `ICAN_IMAGE` in the protected environment file to the new verified digest. Keep `ICAN_DATA_DIR`, `DATA_ENCRYPTION_KEY`, `JWT_SECRET` and saved business settings unchanged.
5. Run the same `config --quiet`, `create --no-build`, policy check and `start` steps above. No server build is involved. Confirm the mount is the original directory and test users, API config decryption, media, canvas and tasks before restoring traffic.

Example cold backup, with paths reviewed beforehand and no other process writing the data:

```sh
docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml stop app
# ICAN_BACKUP_DIR must be a new protected directory outside the application's mount.
umask 077
mkdir -m 700 "$ICAN_BACKUP_DIR"
sudo tar --numeric-owner -cpf "$ICAN_BACKUP_DIR/data.tar" -C "$ICAN_REVIEWED_DATA_DIR" .
# Restrict archive permissions; copy the operator file through your secure backup process.
sudo chmod 600 "$ICAN_BACKUP_DIR/data.tar"
sudo tar -tf "$ICAN_BACKUP_DIR/data.tar" >/dev/null
sudo sha256sum "$ICAN_BACKUP_DIR/data.tar" > "$ICAN_BACKUP_DIR/data.tar.sha256"
```

A checksum proves archive integrity, not recoverability. Test restoration and decryption with the matching key. Protect backups as secrets. Do not use `down -v`, data pruning, or an empty new directory as an upgrade method.

## Rollback

Stop the new app and preserve its current data separately; writes made after the backup may need reconciliation. Never let old and new containers write the same SQLite directory concurrently.

- If the old version is verified compatible with the current schema/config, select the recorded old digest and its matching Compose configuration, then recreate/check/start using the original directory and keys.
- If compatibility is unknown or migrations changed the schema, restore the complete pre-update backup into a **new isolated directory**, verify permissions/checksums and the matching key, and point the old deployment to that restored directory. Preserve the failed-upgrade directory; do not overwrite it in place. Recovery returns to the backup point, so account for any newer writes before reopening service.

Restore the old protected environment/Compose settings (including original keys and correct data path), explicitly pull the old digest if needed, then repeat policy and application checks. A simple image downgrade alone is not a guaranteed database rollback.

## Deployment status

GHCR publication authorizes distribution of the checked image only. It does not authorize an automatic server deployment and does not close the resource-boundary, browser-session, migration, host-firewall or real-account acceptance work in [todo.md](../progress/todo.md). Keep production unchanged until those gates pass.
