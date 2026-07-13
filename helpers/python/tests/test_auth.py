# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 fusion-platform contributors

import io
import json
import os
import unittest
import urllib.error
from unittest import mock

from fusion_runner_helpers.auth import AuthTokenError, KeycloakAuth


def _http_response(payload):
    body = json.dumps(payload).encode("utf-8")
    resp = mock.MagicMock()
    resp.read.return_value = body
    resp.__enter__.return_value = resp
    resp.__exit__.return_value = False
    return resp


class KeycloakAuthTests(unittest.TestCase):
    def setUp(self):
        os.environ["CLIENT_ID"] = "my-client"
        os.environ["CLIENT_SECRET"] = "my-secret"
        os.environ["TOKEN_URL"] = "https://keycloak.example.com/token"

    def tearDown(self):
        for key in ("CLIENT_ID", "CLIENT_SECRET", "TOKEN_URL"):
            os.environ.pop(key, None)

    def test_missing_env_var_raises(self):
        del os.environ["CLIENT_SECRET"]
        with self.assertRaises(AuthTokenError):
            KeycloakAuth()

    @mock.patch("fusion_runner_helpers.auth.urllib.request.urlopen")
    def test_get_token_fetches_and_caches(self, mock_urlopen):
        mock_urlopen.return_value = _http_response({"access_token": "tok-1", "expires_in": 1800})
        auth = KeycloakAuth()

        self.assertEqual(auth.get_token(), "tok-1")
        self.assertEqual(auth.get_token(), "tok-1")
        self.assertEqual(mock_urlopen.call_count, 1)

    @mock.patch("fusion_runner_helpers.auth.urllib.request.urlopen")
    def test_get_token_refreshes_after_expiry(self, mock_urlopen):
        mock_urlopen.side_effect = [
            _http_response({"access_token": "tok-1", "expires_in": 0}),
            _http_response({"access_token": "tok-2", "expires_in": 1800}),
        ]
        auth = KeycloakAuth(expiry_leeway_seconds=0)

        self.assertEqual(auth.get_token(), "tok-1")
        self.assertEqual(auth.get_token(), "tok-2")
        self.assertEqual(mock_urlopen.call_count, 2)

    @mock.patch("fusion_runner_helpers.auth.urllib.request.urlopen")
    def test_default_one_retry_then_raises(self, mock_urlopen):
        mock_urlopen.side_effect = urllib.error.URLError("connection refused")
        auth = KeycloakAuth(retry_backoff_seconds=0)

        with self.assertRaises(AuthTokenError):
            auth.get_token()
        self.assertEqual(mock_urlopen.call_count, 2)  # initial attempt + 1 retry

    @mock.patch("fusion_runner_helpers.auth.urllib.request.urlopen")
    def test_zero_retries_fails_after_one_attempt(self, mock_urlopen):
        mock_urlopen.side_effect = urllib.error.URLError("connection refused")
        auth = KeycloakAuth(max_retries=0, retry_backoff_seconds=0)

        with self.assertRaises(AuthTokenError):
            auth.get_token()
        self.assertEqual(mock_urlopen.call_count, 1)

    @mock.patch("fusion_runner_helpers.auth.urllib.request.urlopen")
    def test_http_error_response_raises(self, mock_urlopen):
        mock_urlopen.side_effect = urllib.error.HTTPError(
            "https://keycloak.example.com/token", 401, "unauthorized",
            hdrs=None, fp=io.BytesIO(b"invalid_client"),
        )
        auth = KeycloakAuth(max_retries=0)

        with self.assertRaises(AuthTokenError):
            auth.get_token()


if __name__ == "__main__":
    unittest.main()
