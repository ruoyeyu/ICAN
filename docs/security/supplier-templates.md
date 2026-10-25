# Supplier templates

[简体中文](supplier-templates.zh-CN.md)

ICAN ships with generic presets only. Each installation manages its own supplier methods. Private supplier catalogs, addresses, model IDs, credentials and remote template feeds are not bundled with the public project.

## Administrator

Open **Admin → Settings → Supplier templates** (`供应商模板`). Choose **New** to enter the name, base address, models and usage notes, then use the existing per-model interface form. **Import** accepts the credential-free `ican-channel` version 1 JSON exported from a personal channel. Review and save it. **Edit**, **Export** and **Delete** act on that one supplier file.

Signed-in users open **Configuration → Local direct → Add custom API**, select the supplier, enter their own key and add the channel. Press **Done** in configuration to save to their account. A new device restores the channel, method and key through existing account synchronization. Selecting a different template clears the draft key.

The catalog provides configuration, not shared administrator credentials. Existing cloud channels remain separate. Custom-channel permissions, request routing and outbound security checks still apply; selecting a template does not force every protocol to run in the browser.

## Files and persistence

Compose sets `API_TEMPLATE_DIR=/app/data/api-templates`, corresponding to `$ICAN_DATA_DIR/api-templates` on the host. Native runs default to `data/api-templates`; `API_TEMPLATE_DIR` may override it. No new database table or service is required.

Each file is `<id>.json`: the ID starts with a letter/digit and contains only letters, digits, underscores or hyphens. The UI assigns a UUID. An operator/local agent can place an exported JSON file in this directory. Write a temporary file first, then rename it to `.json`. Restrict directory writes to the service account/operator (container UID/GID 10001); UI-created files use mode 0600. Never put credentials in JSON, notes or literal fields.

Opening the selector rereads the directory. Files appear/disappear without a restart or rebuild. There are no network template updates, polling intervals, executable plug-ins or dependency installers. Invalid files are excluded and reported to administrators. Symlinks, special files and directory traversal are rejected. Templates reuse the existing 129 MiB configuration boundary; no new disk quota or global concurrency limit is introduced.

The optional `instructions` field is plain text: no HTML rendering, remote images or scripts. Other fields use the existing [portable authoring format](custom-api-templates.md).

## Personal imports and changes

Importing a file in **Add custom API** keeps it as a personal template when adding the channel. It appears under **Personal templates** and follows the account after **Done**; other accounts cannot see it. Select it and choose **Delete personal template**, then **Done**, to sync removal.

Channels retain their own copied configuration. Editing/deleting a catalog entry affects future selections, not existing channels, keys, media or running tasks. Explicitly reconfigure a channel to adopt a changed method. No model, prompt or paid task is silently substituted. Deleting a site template removes its file; there is no second catalog database or background update cache.

## Updates and rollback

Back up the existing data directory, including `api-templates`, with the deployment configuration and original encryption key. Keep the same bind mount and keys. The account-format addition is optional `channelTemplates`; existing IDs/values are preserved. An older client omitting that field does not erase it on this server; explicit `[]` removes personal templates. Old portable files without `instructions` remain valid.

Before reverting to an older image, retain the current database and template directory backup. Older versions cannot display the catalog and may not preserve the optional account field when saving. Restore a matching backup if needed; do not regenerate keys or initialize another database. See [image updates and rollback](ghcr-deployment.md).
