# Supplier templates

[简体中文](supplier-templates.zh-CN.md)

ICAN ships with generic presets only. Each installation manages its own supplier methods. Private supplier catalogs, addresses, model IDs, credentials and remote template feeds are not bundled with the public project.

## Administrator

Open **Admin → Settings → Supplier templates** (`供应商模板`). Choose **New** to enter the name, base address, models and usage notes, then use the existing per-model interface form. **Import** accepts the credential-free `ican-channel` version 1 JSON exported from a personal channel. Review and save it. **Edit**, **Export** and **Delete** act on that one supplier file.

The editor also accepts a **temporary API key** for **Fetch models**. This reuses the existing administrator discovery endpoint and model selector. Search, checkboxes and select/clear-current-list are available. Existing models stay selected; new candidates start unchecked. **Confirm** updates the draft only; **Save template** persists it. Cancel or failed discovery leaves the configuration unchanged. The temporary key is never included in templates, exports or account settings and is cleared when closing the editor or confirming/cancelling selection. Listing models does not verify generation support or run generation tests. Suppliers without a compatible discovery endpoint can still use manual model entry.

Signed-in users open **Configuration → Local direct → Add custom API**, select the supplier, enter their own key and add the channel. Press **Done** in configuration to save to their account. A new device restores the channel, method and key through existing account synchronization. Selecting a different template clears the draft key.

The catalog provides configuration, not shared administrator credentials. Existing cloud channels remain separate. Custom-channel permissions, request routing and outbound security checks still apply; selecting a template does not force every protocol to run in the browser.

## Shared rules and model overrides

In the existing editor, choose **Supplier shared rules** (`供应商公共规则`) under **Rule scope** (`规则范围`) and configure each task type separately: text, image, video or audio. Newly added models inherit the rule for the requested task type. Map the model field to `model`, which always contains the selected model name.

Use **Copy as shared rule** (`复制为公共规则`) to copy an existing model rule while retaining the original. A same-type model-specific rule takes precedence, including its parameter options and media encoding. Rules are complete replacements, not field-by-field merges. Types never fall through to another type. A missing rule in a custom channel stops before submission; unconfigured native-compatible calls retain their normal protocol.

Catalog edits still initialize future channels only. Existing saved channels must explicitly add/import shared rules through their own interface editor. No keys or existing channel values are replaced automatically.

The outer portable format remains `ican-channel` v1. Shared rules use `authoring.version: 2`, with `defaults` and the existing `models` array. Compilation produces the existing engine's `defaults: {text, image, video, audio}` and `models` objects. Unedited v1 channels and advanced rules remain readable without a database migration or new service.

Custom text rules currently accept synchronous JSON/text responses. Map `messages` or the original text `request`; extract a string or a message object containing `content`/`tool_calls`. Custom streaming and asynchronous text are not supported; explicit requests for these are rejected before submission without rewriting `stream`. Native-compatible text streaming is unchanged.

## Files and persistence

Compose sets `API_TEMPLATE_DIR=/app/data/api-templates`, corresponding to `$ICAN_DATA_DIR/api-templates` on the host. Native runs default to `data/api-templates`; `API_TEMPLATE_DIR` may override it. No new database table or service is required.

Each file is `<id>.json`: the ID starts with a letter/digit and contains only letters, digits, underscores or hyphens. The UI assigns a UUID. An operator/local agent can place an exported JSON file in this directory. Write a temporary file first, then rename it to `.json`. Restrict directory writes to the service account/operator (container UID/GID 10001); UI-created files use mode 0600. Never put credentials in JSON, notes or literal fields.

Opening the selector rereads the directory. Files appear/disappear without a restart or rebuild. There are no network template updates, polling intervals, executable plug-ins or dependency installers. Invalid files are excluded and reported to administrators. Symlinks, special files and directory traversal are rejected. Templates reuse the existing 129 MiB configuration boundary; no new disk quota or global concurrency limit is introduced.

The optional `instructions` field is plain text: no HTML rendering, remote images or scripts. Other fields use the existing [portable authoring format](custom-api-templates.md).

## Optional parameter sending

Use the existing field mapping's **Omit when empty** checkbox and **Omit specified values** input. The list starts empty. For example, keep `resolution` as the source, enable empty omission and add `auto`: the field exports `"optional": true, "omitValues": ["auto"]`. Empty values and the string `auto` are omitted; other values remain unchanged. This is configured per field, never enabled globally for suppliers.

Empty means missing, `null`, an empty string or an empty array, as before. Zero and false are preserved unless explicitly listed. Matches are type-, case- and whitespace-sensitive and happen before the existing string conversion. Enter `0` / `false` for a number/boolean, or `"0"` / `"false"` for strings. Body/query fields, fixed JSON values, UI variables, shared rules and model overrides use the same mechanism.

Editing, import/export and account sync use the existing workflow. Updating a catalog template does not replace saved channels; explicitly edit their corresponding fields to adopt the rule.

## Personal imports and changes

Importing a file in **Add custom API** keeps it as a personal template when adding the channel. It appears under **Personal templates** and follows the account after **Done**; other accounts cannot see it. Select it and choose **Delete personal template**, then **Done**, to sync removal.

Channels retain their own copied configuration. Editing/deleting a catalog entry affects future selections, not existing channels, keys, media or running tasks. Explicitly reconfigure a channel to adopt a changed method. No model, prompt or paid task is silently substituted. Deleting a site template removes its file; there is no second catalog database or background update cache.

## Updates and rollback

Back up the existing data directory, including `api-templates`, with the deployment configuration and original encryption key. Keep the same bind mount and keys. The account-format addition is optional `channelTemplates`; existing IDs/values are preserved. An older client omitting that field does not erase it on this server; explicit `[]` removes personal templates. Old portable files without `instructions` remain valid.

Before reverting to an older image, retain the current database and template directory backup. Older versions cannot display the catalog and may not preserve the optional account field when saving. Restore a matching backup if needed; do not regenerate keys or initialize another database. See [image updates and rollback](ghcr-deployment.md).

Before enabling shared rules, back up the database, media, `api-templates` and original encryption key. Only explicitly saved configurations adopt v2. To roll back to an image without v2 support, restore the matching database/template backup; do not resave v2 channels with the older application. Existing video tasks retain their saved rule snapshots and are never recreated because a template changes.

Omission rules add an optional field without a database migration. Templates without it retain their exact v1/v2 compilation. Back up channel configurations and template files before opting in. Older images reject `omitValues`; restore matching pre-change configurations when rolling back and do not resave extended templates in an older editor.
