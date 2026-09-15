# config.sh

Utilities for loading configuration from files and environment variables.

> 🧭 Source: [src/config.sh](../src/config.sh)
>
> Jump to: [Overview](#overview) · [Tips](#tips) · [Reference](#reference)

<a id="overview"></a>
## ✨ Overview

Configuration files are loaded in the order provided, so later files
override earlier files. Environment variables loaded with
`dybatpho::config_env` are applied last.


Keys can also be given a typed schema with `dybatpho::config_schema`.
`dybatpho::config_validate` then applies declared defaults, enforces
required keys, types, ranges, and enum choices, and reports every
violation together with the key that caused it. The same schema renders a
configuration reference through `dybatpho::config_doc`.

### 🚀 Highlights

- [`__dybatpho_config_set`](#__dybatpho_config_set) — 
- [`__dybatpho_config_load_dotenv`](#__dybatpho_config_load_dotenv) — 
- [`__dybatpho_config_load_structured`](#__dybatpho_config_load_structured) — 
- [`dybatpho::config_load`](#dybatphoconfig_load) — Load one or more configuration files.
- [`dybatpho::config_env`](#dybatphoconfig_env) — Load environment variables after an optional prefix.
- [`dybatpho::config_get`](#dybatphoconfig_get) — Print a configuration value.
- [`dybatpho::config_require`](#dybatphoconfig_require) — Require configuration keys to be present.
- [`dybatpho::config_export`](#dybatphoconfig_export) — Export loaded values as shell variables.
- [`__dybatpho_config_schema_type`](#__dybatpho_config_schema_type) — Normalize a schema type name to its canonical form.
- [`__dybatpho_config_schema_clear`](#__dybatpho_config_schema_clear) — Drop every attribute previously declared for a key.
- [`__dybatpho_config_schema_attr`](#__dybatpho_config_schema_attr) — Print a schema attribute, or a fallback when it is not declared.
- [`__dybatpho_config_schema_constraints`](#__dybatpho_config_schema_constraints) — Describe the range and choice constraints declared for a key.
- [`dybatpho::config_schema`](#dybatphoconfig_schema) — Declare validation rules for a configuration key.
- [`dybatpho::config_schema_reset`](#dybatphoconfig_schema_reset) — Forget every declared configuration schema.
- [`__dybatpho_config_schema_error`](#__dybatpho_config_schema_error) — Record a validation failure for a configuration key.
- [`__dybatpho_config_schema_check`](#__dybatpho_config_schema_check) — Validate a single value against the type declared for its key.
- [`dybatpho::config_validate`](#dybatphoconfig_validate) — Validate configured values against all declared schemas. Missing optional keys take their declared default, and every violation is reported with the key that caused it.
- [`__dybatpho_config_doc_cell`](#__dybatpho_config_doc_cell) — Render one Markdown table cell, escaping pipes and marking empties.
- [`__dybatpho_config_doc_json_value`](#__dybatpho_config_doc_json_value) — Render one JSON value, emitting `null` for an undeclared attribute.
- [`dybatpho::config_doc`](#dybatphoconfig_doc) — Render documentation for every declared configuration key.

<a id="tips"></a>
## 💡 Tips

### `dybatpho::config_env`

- Environment variables override values loaded from configuration files.

### `dybatpho::config_schema`

- Call `dybatpho::config_validate` after all files and environment overlays are loaded.
- Declaring the same key twice replaces its previous rules instead of merging them.

### `dybatpho::config_doc`

- Pipe the Markdown output into a `CONFIGURATION.md` file to keep docs in sync with the schema.

<a id="reference"></a>
## 📚 Reference

### `__dybatpho_config_set`



---

### `__dybatpho_config_load_dotenv`



---

### `__dybatpho_config_load_structured`



---

### `dybatpho::config_load`

Load one or more configuration files.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$@` | string | Files in dotenv, JSON, or YAML format, in increasing precedence order |

**🚦 Exit codes**

- `1`: A file is missing or has invalid configuration


---

### `dybatpho::config_env`

Load environment variables after an optional prefix.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Optional prefix, such as `APP_` |


---

### `dybatpho::config_get`

Print a configuration value.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |
| `$2` | string | Optional default value |

**📤 Output on stdout**

- Configuration value

**🚦 Exit codes**

- `1`: Key is missing and no default was supplied


---

### `dybatpho::config_require`

Require configuration keys to be present.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$@` | string | Configuration keys |

**🚦 Exit codes**

- `1`: At least one key is missing


---

### `dybatpho::config_export`

Export loaded values as shell variables.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Optional prefix for exported variable names |

**🚦 Exit codes**

- `1`: A key cannot be represented as a shell variable


---

### `__dybatpho_config_schema_type`

Normalize a schema type name to its canonical form.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Declared type |

**📤 Output on stdout**

- Canonical type: string, int, bool, url, or enum

**🚦 Exit codes**

- `1`: The type is not supported


---

### `__dybatpho_config_schema_clear`

Drop every attribute previously declared for a key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |


---

### `__dybatpho_config_schema_attr`

Print a schema attribute, or a fallback when it is not declared.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |
| `$2` | string | Attribute name |
| `$3` | string | Optional fallback value |

**📤 Output on stdout**

- Attribute value


---

### `__dybatpho_config_schema_constraints`

Describe the range and choice constraints declared for a key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |

**📤 Output on stdout**

- Human readable constraints, or an empty string when none are declared


---

### `dybatpho::config_schema`

Declare validation rules for a configuration key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |
| `$2` | string | Type: `string`, `int` (`integer`), `bool` (`boolean`), `url`, or `enum` |
| `$@` | string | Rules: `required:true`, `default:value`, `min:number`, `max:number`, `choices:a,b`, `description:text` |

**🧩 Variable sets**

- **`DYBATPHO_CONFIG_SCHEMA`**: Declared attributes, keyed by `<key>.<attribute>`
- **`DYBATPHO_CONFIG_SCHEMA_KEYS`**: Declaration order used by validation and documentation

**🚦 Exit codes**

- `1`: The key, type, or a rule is invalid


---

### `dybatpho::config_schema_reset`

Forget every declared configuration schema.

_Function has no arguments._

**🧩 Variable sets**

- **`DYBATPHO_CONFIG_SCHEMA`**: Emptied
- **`DYBATPHO_CONFIG_SCHEMA_KEYS`**: Emptied


---

### `__dybatpho_config_schema_error`

Record a validation failure for a configuration key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |
| `$2` | string | Reason describing the violation |

**🧩 Variable sets**

- **`DYBATPHO_CONFIG_ERRORS`**: Appends the formatted message


---

### `__dybatpho_config_schema_check`

Validate a single value against the type declared for its key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Configuration key |
| `$2` | string | Effective value |

**🧩 Variable sets**

- **`DYBATPHO_CONFIG_ERRORS`**: Appends one message per violation


---

### `dybatpho::config_validate`

Validate configured values against all declared schemas.
  Missing optional keys take their declared default, and every violation is
  reported with the key that caused it.

_Function has no arguments._

**🧩 Variable sets**

- **`DYBATPHO_CONFIG`**: Applies declared defaults for missing keys
- **`DYBATPHO_CONFIG_ERRORS`**: One message per violation, in declaration order

**🚦 Exit codes**

- `1`: A required key is missing or a value violates its schema


---

### `__dybatpho_config_doc_cell`

Render one Markdown table cell, escaping pipes and marking empties.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Cell text |
| `$2` | string | Optional `code` to wrap a non-empty cell in backticks |

**📤 Output on stdout**

- Markdown cell text, or `-` when the value is empty


---

### `__dybatpho_config_doc_json_value`

Render one JSON value, emitting `null` for an undeclared attribute.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Attribute value |
| `$2` | string | Optional `declared` to emit an empty string instead of `null` |

**📤 Output on stdout**

- Quoted JSON string, or `null`


---

### `dybatpho::config_doc`

Render documentation for every declared configuration key.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Optional format: `markdown` (default), `text`, or `json` |
| `$2` | string | Optional title used by the `markdown` and `text` formats |

**📤 Output on stdout**

- Configuration reference in the requested format

**🚦 Exit codes**

- `1`: The format is not supported

