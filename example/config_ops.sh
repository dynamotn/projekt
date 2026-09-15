#!/usr/bin/env bash
# @file config_ops.sh
# @brief Example showing layered configuration and schema validation.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh"

dybatpho::register_common_handlers

config_dir="${TMPDIR:-/tmp}/dybatpho-config-example-${BASHPID}"
mkdir -p "${config_dir}"
trap 'rm -rf -- "${config_dir}"' EXIT

printf 'HOST=localhost\nPORT=8080\nMODE=dev\n' > "${config_dir}/defaults.env"
printf 'PORT=8443\n' > "${config_dir}/local.env"

# Environment values have the highest precedence over configuration files.
export APP_MODE=prod
dybatpho::config_load "${config_dir}/defaults.env" "${config_dir}/local.env"
dybatpho::config_env APP_

# Every key is declared with a type, its constraints, and a description that
# feeds both the error messages and the generated reference.
dybatpho::config_schema HOST string required:true min:1 \
  description:"Service hostname"
dybatpho::config_schema PORT integer min:1 max:65535 \
  description:"Listening port"
dybatpho::config_schema MODE enum choices:dev,prod \
  description:"Deployment mode"
dybatpho::config_schema REGION string default:us-east-1 \
  description:"Cloud region"
dybatpho::config_schema ENDPOINT url default:https://api.example.test \
  description:"Upstream API endpoint"
dybatpho::config_validate
dybatpho::config_export APP_

dybatpho::header "CONFIGURATION"
dybatpho::print "host: ${APP_HOST}"
dybatpho::print "port: ${APP_PORT}"
dybatpho::print "mode: ${APP_MODE}"
dybatpho::print "region (default): ${APP_REGION}"
dybatpho::print "endpoint (default): ${APP_ENDPOINT}"
dybatpho::success "Configuration validated"

dybatpho::header "CONFIGURATION REFERENCE"
dybatpho::config_doc markdown "Example settings"

# Validation failures are reported per key, so every problem surfaces at once
# instead of one error per run.
dybatpho::header "VALIDATION FAILURES"
__dybatpho_config_set PORT "not-a-number"
__dybatpho_config_set MODE staging
unset "DYBATPHO_CONFIG[HOST]"
if (dybatpho::config_validate); then
  dybatpho::error "Expected the invalid configuration to be rejected"
else
  dybatpho::warn "Rejected the invalid configuration as expected"
fi
