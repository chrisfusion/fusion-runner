#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENHANCE_FILE="${SCRIPT_DIR}/enhance_docker_ssl"

# Build the certify stage body (FROM line always added by caller).
# When enhance_docker_ssl exists its content is appended verbatim (curl calls etc).
# When absent a minimal ca-certificates fallback is used.
build_certify_stage() {
    echo "FROM ubuntu:24.04 AS certify"
    if [[ -f "${ENHANCE_FILE}" ]]; then
        cat "${ENHANCE_FILE}"
    else
        printf 'RUN apt-get update -qq \\\n'
        printf '    && apt-get install -y --no-install-recommends ca-certificates \\\n'
        printf '    && update-ca-certificates \\\n'
        printf '    && rm -rf /var/lib/apt/lists/*\n'
    fi
}

# Replace a placeholder line or an existing START…END sentinel block in a file.
# Usage: inject_block <placeholder> <start_marker> <end_marker> <inject_file> <input> <output>
inject_block() {
    local ph="$1" sm="$2" em="$3" inject="$4" input="$5" output="$6"
    awk \
        -v ph="${ph}" \
        -v sm="${sm}" \
        -v em="${em}" \
        -v inject="${inject}" \
        '
        $0 == ph || $0 == sm {
            while ((getline line < inject) > 0) print line
            close(inject)
            if ($0 == sm) skip = 1
            next
        }
        $0 == em && skip { skip = 0; next }
        skip { next }
        { print }
        ' "${input}" > "${output}"
}

mapfile -t DOCKERFILES < <(find "${SCRIPT_DIR}/dockerfiles" -name "Dockerfile" -type f | sort)

if [[ ${#DOCKERFILES[@]} -eq 0 ]]; then
    echo "[prepare_Dockerfile] No Dockerfiles found under ${SCRIPT_DIR}/dockerfiles" >&2
    exit 1
fi

for DOCKERFILE in "${DOCKERFILES[@]}"; do
    echo "[prepare_Dockerfile] Processing: ${DOCKERFILE}"

    cp "${DOCKERFILE}" "${DOCKERFILE}_bak"

    # Assemble the two injection blocks
    CERTIFY_STAGE="$(build_certify_stage)"

    BUILDER_TMPFILE="$(mktemp)"
    COPY_TMPFILE="$(mktemp)"
    WORK="$(mktemp)"

    # Write sentinel-wrapped certify builder block
    {
        echo "# ##CERTIFY_BUILDER_START##"
        printf '%s\n' "${CERTIFY_STAGE}"
        echo "# ##CERTIFY_BUILDER_END##"
    } > "${BUILDER_TMPFILE}"

    # Write sentinel-wrapped certify copy block
    {
        echo "# ##CERTIFY_COPY_START##"
        echo "COPY --from=certify /etc/ssl/certs /etc/ssl/certs"
        echo "# ##CERTIFY_COPY_END##"
    } > "${COPY_TMPFILE}"

    # Step 1: inject certify builder (Dockerfile → WORK)
    inject_block \
        "# ##CERTIFY_BUILDER##" \
        "# ##CERTIFY_BUILDER_START##" \
        "# ##CERTIFY_BUILDER_END##" \
        "${BUILDER_TMPFILE}" \
        "${DOCKERFILE}" \
        "${WORK}"

    # Step 2: inject certify copy (WORK → Dockerfile)
    inject_block \
        "# ##CERTIFY_COPY##" \
        "# ##CERTIFY_COPY_START##" \
        "# ##CERTIFY_COPY_END##" \
        "${COPY_TMPFILE}" \
        "${WORK}" \
        "${DOCKERFILE}"

    rm -f "${BUILDER_TMPFILE}" "${COPY_TMPFILE}" "${WORK}"

    echo "[prepare_Dockerfile] Done: ${DOCKERFILE} (backup: ${DOCKERFILE}_bak)"
done
