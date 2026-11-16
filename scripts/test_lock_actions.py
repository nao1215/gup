"""An upstream generator must not silently relax the workflow SHA policy."""
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import lock_actions


class LockActionsTests(unittest.TestCase):
    def test_sha_workflow_keeps_exact_lockfile_key(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            workflows = root/'.github/workflows'; workflows.mkdir(parents=True)
            path = workflows/'test.yml'
            sha = 'a'*40
            original = f'uses: actions/checkout@{sha}\n'
            path.write_text(original)
            lock = workflows/'actions.lock'
            def generate(args, **kwargs):
                if '--verify' in args: return
                path.write_text('uses: actions/checkout@v7\n')
                lock.write_text("version: 'v0.0.3'\nworkflows:\n    '.github/workflows/test.yml':\n        - 'actions/checkout@v7'\ndependencies:\n    'actions/checkout@v7':\n        ref: 'v7'\n        commit: 'sha1-" + sha + "'\n        owner_id: 1\n        repo_id: 2\n")
            with patch.object(lock_actions, 'ROOT', root), patch.object(lock_actions.subprocess, 'run', side_effect=generate):
                lock_actions.main()
            self.assertEqual(path.read_text(), original)
            self.assertIn(f"    'actions/checkout@{sha}':", lock.read_text())
            self.assertIn(f"        - 'actions/checkout@{sha}'", lock.read_text())

    def test_tag_alias_is_not_exact_runtime_coverage(self):
        text = "version: 'v0.0.3'\nworkflows:\n    '.github/workflows/test.yml':\n        - 'actions/checkout@v7'\ndependencies:\n    'actions/checkout@v7':\n        ref: 'v7'\n        commit: 'sha1-" + 'a'*40 + "'\n        owner_id: 1\n        repo_id: 2\n"
        expected = {'.github/workflows/test.yml': {'actions/checkout@'+'a'*40}}
        with self.assertRaisesRegex(ValueError, 'workflow references differ'):
            lock_actions.check_exact_refs(text, expected)
        converted = lock_actions.retain_sha_keys(text, expected)
        lock_actions.check_exact_refs(converted, expected)
        self.assertIn("ref: 'v7'", converted)
        self.assertIn('owner_id: 1\n        repo_id: 2', converted)
        with self.assertRaisesRegex(ValueError, 'did not resolve'):
            lock_actions.retain_sha_keys(text.replace('a'*40, 'b'*40), expected)
        with self.assertRaisesRegex(ValueError, 'exact SHA dependency'):
            lock_actions.check_exact_refs(converted.replace("commit: 'sha1-"+'a'*40, "commit: 'sha1-"+'b'*40), expected)

    def test_subpath_and_owner_case_share_repository_pin(self):
        root = lock_actions.ROOT
        originals = {root/'.github/workflows/test.yml': ('uses: k1LoW/action/step@'+'a'*40+'\n  uses: ./.github/workflows/local.yml\n').encode()}
        self.assertEqual(lock_actions.workflow_refs(originals), {'.github/workflows/test.yml': {'k1low/action@'+'a'*40}})

    def test_reusable_workflow_call_is_not_an_action_pin(self):
        root = lock_actions.ROOT
        originals = {root/'.github/workflows/test.yml': ('uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.1.0\n').encode()}
        self.assertEqual(lock_actions.workflow_refs(originals), {'.github/workflows/test.yml': set()})

    def test_restore_sha_even_when_generator_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            workflows = root/'.github/workflows'; workflows.mkdir(parents=True)
            path = workflows/'test.yml'
            original = b'uses: actions/checkout@' + b'a'*40 + b'\n'
            path.write_bytes(original)
            def generate(*args, **kwargs):
                path.write_text('uses: actions/checkout@v7\n')
                raise subprocess.CalledProcessError(1, args[0])
            with patch.object(lock_actions, 'ROOT', root), patch.object(lock_actions.subprocess, 'run', side_effect=generate):
                with self.assertRaises(subprocess.CalledProcessError):
                    lock_actions.main()
            self.assertEqual(path.read_bytes(), original)


if __name__ == '__main__':
    unittest.main()
