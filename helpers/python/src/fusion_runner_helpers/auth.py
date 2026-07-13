# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 fusion-platform contributors

import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request


class AuthTokenError(RuntimeError):
    """Raised when an access token cannot be obtained."""


class KeycloakAuth:
    """OAuth2 client-credentials token fetcher with in-process caching.

    Reads the client id, client secret, and token endpoint URL from environment
    variables. Key names are configurable since the injected Secret may not use
    the platform default names (CLIENT_ID, CLIENT_SECRET, TOKEN_URL).
    """

    def __init__(
        self,
        client_id_key="CLIENT_ID",
        client_secret_key="CLIENT_SECRET",
        token_url_key="TOKEN_URL",
        max_retries=1,
        retry_backoff_seconds=1.0,
        expiry_leeway_seconds=30,
        timeout_seconds=10,
    ):
        self._client_id = _require_env(client_id_key)
        self._client_secret = _require_env(client_secret_key)
        self._token_url = _require_env(token_url_key)
        self._max_retries = max_retries
        self._retry_backoff_seconds = retry_backoff_seconds
        self._expiry_leeway_seconds = expiry_leeway_seconds
        self._timeout_seconds = timeout_seconds

        self._access_token = None
        self._expires_at = 0.0

    def get_token(self):
        """Returns a currently-valid access token, refreshing first if expired."""
        if self._access_token is None or time.monotonic() >= self._expires_at:
            self._refresh()
        return self._access_token

    def _refresh(self):
        last_error = None
        for attempt in range(self._max_retries + 1):
            try:
                token, expires_in = self._fetch_token()
                self._access_token = token
                self._expires_at = time.monotonic() + max(expires_in - self._expiry_leeway_seconds, 0)
                return
            except Exception as exc:
                last_error = exc
                if attempt < self._max_retries:
                    time.sleep(self._retry_backoff_seconds)
        raise AuthTokenError(
            f"failed to obtain access token from {self._token_url} "
            f"after {self._max_retries + 1} attempt(s): {last_error}"
        ) from last_error

    def _fetch_token(self):
        data = urllib.parse.urlencode({
            "grant_type": "client_credentials",
            "client_id": self._client_id,
            "client_secret": self._client_secret,
        }).encode("utf-8")

        req = urllib.request.Request(
            self._token_url,
            data=data,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout_seconds) as resp:
                payload = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            raise AuthTokenError(f"token endpoint returned {exc.code}: {body}") from exc
        except urllib.error.URLError as exc:
            raise AuthTokenError(f"token endpoint unreachable: {exc.reason}") from exc

        access_token = payload.get("access_token")
        expires_in = payload.get("expires_in")
        if not access_token or expires_in is None:
            raise AuthTokenError(f"token endpoint response missing access_token/expires_in: {payload}")
        return access_token, float(expires_in)


def _require_env(key):
    value = os.environ.get(key)
    if not value:
        raise AuthTokenError(f"required environment variable {key!r} is not set")
    return value
