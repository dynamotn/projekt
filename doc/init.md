# init.sh

Initial script

> 🧭 Source: [init.sh](../init.sh)
>
> Jump to: [Overview](#overview) · [See also](#see-also) · [Reference](#reference)

<a id="overview"></a>
## ✨ Overview

This script should be sourced before any of
the other scripts in this repo. Other scripts
make use of ${DYBATPHO_DIR} to find each other.


  Sourcing `init.sh` with no arguments loads the core modules only. A script
  names the modules it needs, and dybatpho resolves the dependencies for it:


  ```sh
  . dybatpho/init.sh --modules logging git semver   # positional form
  DYBATPHO_MODULES="logging git semver" . dybatpho/init.sh # environment form
  . dybatpho/init.sh --modules all                  # the whole library
  ```


  Every module set includes the core modules, and can be widened later with
  `dybatpho::load`.



### 🌍 Environment

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_CORE_MODULES`** | string | Modules that call each other and are always loaded |
| **`DYBATPHO_OPTIONAL_MODULES`** | string | Modules that are only loaded when requested |
| **`DYBATPHO_LOADED_MODULES`** | string | Modules loaded so far, in load order |

### 🚀 Highlights

- [`__dybatpho_source_module`](#__dybatpho_source_module) — Source one module file. Every module lives at `src/<name>.sh`, so the name is all the map that is needed.
- [`__dybatpho_module_exists`](#__dybatpho_module_exists) — Return success when a name is a known dybatpho module.
- [`__dybatpho_load_module`](#__dybatpho_load_module) — Source a module and its dependencies, at most once each.
- [`__dybatpho_export_functions`](#__dybatpho_export_functions) — Filter functions and re-export only dybatpho functions to subshells.
- [`dybatpho::load`](#dybatphoload) — Load one or more modules after `init.sh` has already been sourced.
- [`dybatpho::module_loaded`](#dybatphomodule_loaded) — Return success when a module is already loaded.
- [`dybatpho::module_list`](#dybatphomodule_list) — Print module names, one per line.

<a id="see-also"></a>
## 🔗 See also

- [example/init_modules.sh](../example/init_modules.sh)

<a id="reference"></a>
## 📚 Reference

### `__dybatpho_source_module`

Source one module file. Every module lives at
  `src/<name>.sh`, so the name is all the map that is needed.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Module name |

**🚦 Exit codes**

- `0`: The module file is sourced
- `1`: The module is registered but has no file under `src/`


---

### `__dybatpho_module_exists`

Return success when a name is a known dybatpho module.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Module name |

**🚦 Exit codes**

- `0`: The module is part of the registry
- `1`: The module is unknown


---

### `__dybatpho_load_module`

Source a module and its dependencies, at most once each.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Module name |

**🧩 Variable sets**

- DYBATPHO_LOADED_MODULES

**🚦 Exit codes**

- `0`: The module and its dependencies are loaded
- `1`: The module is unknown


---

### `__dybatpho_export_functions`

Filter functions and re-export only dybatpho functions to subshells.

_Function has no arguments._


---

### `dybatpho::load`

Load one or more modules after `init.sh` has already been sourced.

**🧪 Example**

```bash
. dybatpho/init.sh --modules logging
dybatpho::load json config

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$@` | string | Module names; dependencies are resolved automatically |

**🧩 Variable sets**

- DYBATPHO_LOADED_MODULES

**🚦 Exit codes**

- `0`: Every requested module is loaded
- `1`: Stop the script when no module is given or a module is unknown


---

### `dybatpho::module_loaded`

Return success when a module is already loaded.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Module name |

**🚦 Exit codes**

- `0`: The module is loaded
- `1`: The module is not loaded


---

### `dybatpho::module_list`

Print module names, one per line.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Selection, one of `loaded` (default), `all`, `core`, or `optional` |

**📤 Output on stdout**

- Module names in registry order, or in load order for `loaded`

**🚦 Exit codes**

- `0`: Print the requested list
- `1`: Stop the script when the selection is unknown

