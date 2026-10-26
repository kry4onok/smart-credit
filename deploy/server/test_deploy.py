import io
import os
from pathlib import Path
import stat
import tarfile
import tempfile
import unittest

import deploy


class ArchiveTests(unittest.TestCase):
    def archive(self, folder, name="proto/generated/file.py", revision="a" * 40):
        path = folder / "source.tar"
        with tarfile.open(path, "w", pax_headers={"comment": revision}) as archive:
            entry = tarfile.TarInfo(name)
            content = b"# generated source\n"
            entry.mode, entry.size = 0o644, len(content)
            archive.addfile(entry, io.BytesIO(content))
        return path

    def test_non_root_images_can_read_extracted_source(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            archive = self.archive(root)
            previous = os.umask(0o077)
            try:
                deploy.extract_source(archive, root / "release", "a" * 40)
                self.assertEqual(stat.S_IMODE((root / "release/proto/generated").stat().st_mode), 0o755)
                self.assertEqual(os.umask(0o077), 0o077)
            finally:
                os.umask(previous)

    def test_wrong_revision_is_rejected_before_extraction(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaisesRegex(RuntimeError, "revision"):
                deploy.extract_source(self.archive(root), root / "release", "b" * 40)
            self.assertFalse((root / "release").exists())

    def test_archive_cannot_escape_release_directory(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaises(tarfile.FilterError):
                deploy.extract_source(self.archive(root, "../escaped"), root / "release", "a" * 40)
            self.assertFalse((root / "escaped").exists())

    def test_patches_keep_source_readable_and_are_idempotent(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = root / "file.py"
            source.write_text("before\n")
            source.chmod(0o644)
            patch = root / "change.patch"
            patch.write_text("--- a/file.py\n+++ b/file.py\n@@ -1 +1 @@\n-before\n+after\n")
            previous = os.umask(0o077)
            try:
                deploy.apply_patches(root, [patch])
                self.assertEqual(stat.S_IMODE(source.stat().st_mode), 0o644)
                deploy.apply_patches(root, [patch])
                self.assertEqual(source.read_text(), "after\n")
                self.assertEqual(os.umask(0o077), 0o077)
            finally:
                os.umask(previous)


if __name__ == "__main__":
    unittest.main()
