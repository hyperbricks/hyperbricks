#!/usr/bin/env python3
"""Tests for the HyperBricks Codex plugin packaging preflight."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import unittest


SCRIPT = Path(__file__).with_name("package_codex_plugin.py")
SPEC = importlib.util.spec_from_file_location("package_codex_plugin", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
PACKAGE_CODEX_PLUGIN = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PACKAGE_CODEX_PLUGIN)


def manifest_with_short_description(short_description: str) -> bytes:
    return json.dumps(
        {
            "name": "hyperbricks",
            "version": "0.1.0",
            "description": "Develop HyperBricks applications.",
            "interface": {"shortDescription": short_description},
        }
    ).encode("utf-8")


def manifest_with_version(version: str) -> bytes:
    manifest = json.loads(manifest_with_short_description("HyperBricks"))
    manifest["version"] = version
    return json.dumps(manifest).encode("utf-8")


class ValidateManifestTests(unittest.TestCase):
    def test_accepts_plain_numeric_plugin_version(self) -> None:
        manifest = PACKAGE_CODEX_PLUGIN.validate_manifest(
            manifest_with_version("0.1.2")
        )
        self.assertEqual(manifest["version"], "0.1.2")

    def test_rejects_build_metadata_and_prerelease_versions(self) -> None:
        for version in ("0.1.1+codex.20261004160647", "0.1.2-beta.1"):
            with self.subTest(version=version), self.assertRaisesRegex(
                PACKAGE_CODEX_PLUGIN.PackageError,
                "plain MAJOR.MINOR.PATCH",
            ):
                PACKAGE_CODEX_PLUGIN.validate_manifest(
                    manifest_with_version(version)
                )

    def test_accepts_public_short_description_limit(self) -> None:
        manifest = PACKAGE_CODEX_PLUGIN.validate_manifest(
            manifest_with_short_description("x" * 30)
        )

        self.assertEqual(
            manifest["interface"]["shortDescription"],
            "x" * 30,
        )

    def test_rejects_public_short_description_over_limit(self) -> None:
        with self.assertRaisesRegex(
            PACKAGE_CODEX_PLUGIN.PackageError,
            "at most 30 characters for public submission",
        ):
            PACKAGE_CODEX_PLUGIN.validate_manifest(
                manifest_with_short_description("x" * 31)
            )

    def test_rejects_multiline_short_description(self) -> None:
        with self.assertRaisesRegex(
            PACKAGE_CODEX_PLUGIN.PackageError,
            "must fit on one line",
        ):
            PACKAGE_CODEX_PLUGIN.validate_manifest(
                manifest_with_short_description("Develop HyperBricks\napplications")
            )


if __name__ == "__main__":
    unittest.main()
