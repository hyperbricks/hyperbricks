"""Catalog image URL refresh contracts, exercised by the Go regression suite."""
import importlib.util
import io
from pathlib import Path
import threading
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('catalog_demo', Path(__file__).parent.parent / 'demo-api/server.py')
server = importlib.util.module_from_spec(spec)
spec.loader.exec_module(server)


def manifest(version, missing=False, invalid=False):
    ids = list(server.ITEMS)
    if missing:
        ids.pop()
    return ''.join(f'<span data-image-id="{key}"><img src="{("https://outside.test/image.png" if invalid else "/static/images/"+key+"-"+version+".jpg")}"></span>' for key in ids).encode()


class ImageManifestTests(unittest.TestCase):
    def test_refreshes_after_initial_use(self):
        with patch.object(server, 'urlopen', side_effect=[io.BytesIO(manifest('first')), io.BytesIO(manifest('second'))]) as fetch:
            server.ensure_images()
            server.ensure_images()
        self.assertEqual(fetch.call_count, 2)
        self.assertTrue(all(item['image'].endswith('-second.jpg') for item in server.DATA))

    def test_invalid_refresh_preserves_complete_previous_manifest(self):
        for invalid in (False, True):
            with patch.object(server, 'urlopen', return_value=io.BytesIO(manifest('good'))):
                server.ensure_images()
            previous = [dict(item) for item in server.DATA]
            with patch.object(server, 'urlopen', return_value=io.BytesIO(manifest('bad', missing=not invalid, invalid=invalid))):
                with self.assertRaises(ValueError):
                    server.ensure_images()
            self.assertEqual(server.DATA, previous)

    def test_unavailable_refresh_is_reported(self):
        previous = [dict(item) for item in server.DATA]
        with patch.object(server, 'urlopen', side_effect=OSError('unavailable')):
            with self.assertRaises(OSError):
                server.ensure_images()
        self.assertEqual(server.DATA, previous)

    def test_concurrent_refreshes_publish_complete_manifests(self):
        responses = [io.BytesIO(manifest(str(i))) for i in range(8)]
        with patch.object(server, 'urlopen', side_effect=responses) as fetch:
            threads = [threading.Thread(target=server.ensure_images) for _ in responses]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join(timeout=5)
                self.assertFalse(thread.is_alive())
        self.assertEqual(fetch.call_count, 8)
        self.assertTrue(all(item['image'].endswith('-7.jpg') for item in server.DATA))


if __name__ == '__main__':
    unittest.main()
