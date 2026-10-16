"""Verify deployment failure isolation without touching a Docker host."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class ReleaseTest(unittest.TestCase):
    def exercise(self, fail="", different_revision=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            release = root / "deploy/releases/test"
            release.mkdir(parents=True)
            (root / ".env").write_text("PUBLIC_BASE_URL=https://fitlog.example\n")
            (release / "release.sh").write_text(Path(__file__).with_name("release.sh").read_text())
            (release / "docker-compose.shared.yml").write_text("services: {}\n")
            bin_dir = root / "bin"
            bin_dir.mkdir()
            log = root / "commands"
            programs = {
                "docker": '''#!/usr/bin/env python3
import os,sys,json
a=sys.argv[1:]
with open(os.environ['COMMAND_LOG'],'a') as f:f.write(json.dumps(a)+'\\n')
if a[-1]==os.environ.get('FAIL_STAGE'):sys.exit(1)
if a[:2]==['image','inspect']:
 repo=a[2].split('@')[0].split(':')[0]
 print(repo+'@sha256:'+'b'*64)
if 'json' in a:print(json.dumps({'services':{'app':{'ports':[{'published':'18082'}],'environment':{'PUBLIC_BASE_URL':'https://fitlog.example'}}}}))
''',
                "curl": '''#!/usr/bin/env python3
import json,os,sys
with open(os.environ['COMMAND_LOG'],'a') as f:f.write(json.dumps(['curl']+sys.argv[1:])+'\\n')
if 'authorization-server' in sys.argv[-1]:print(json.dumps({'issuer':'https://fitlog.example'}))
elif '%{http_code}' in sys.argv:print('401',end='')
''',
                "flock": "#!/usr/bin/env bash\nexit 0\n",
            }
            for name, content in programs.items():
                path = bin_dir / name
                path.write_text(content)
                path.chmod(0o700)
            env = {**os.environ, "PATH": str(bin_dir)+os.pathsep+os.environ["PATH"], "COMMAND_LOG": str(log), "FAIL_STAGE": fail}
            result = subprocess.run(["bash", str(release / "release.sh"), str(root), "ghcr.io/kenniele/fitlog:"+"a"*40,
                                     "ghcr.io/kenniele/fitlog-web:"+("c" if different_revision else "a")*40], env=env, capture_output=True, text=True)
            commands = [json.loads(x) for x in log.read_text().splitlines()] if log.exists() else []
            return result, commands, (root / "deploy/current").is_symlink()

    def test_success_checks_target_and_never_touches_proxy(self):
        result, commands, current = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(current)
        migration = next(i for i,c in enumerate(commands) if c[-1]=="migrate")
        start = next(i for i,c in enumerate(commands) if "up" in c and "app" in c)
        self.assertLess(migration, start)
        self.assertTrue(any("--resolve" in c and "fitlog.example:443:127.0.0.1" in c for c in commands))
        for command in commands:
            self.assertFalse(set(command) & {"down", "--remove-orphans", "caddy", "prune"})

    def test_failed_migration_does_not_replace_app(self):
        result, commands, current = self.exercise("migrate")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(current)
        self.assertFalse(any("up" in c and "app" in c for c in commands))

    def test_mixed_revisions_rejected_before_docker(self):
        result, commands, current = self.exercise(different_revision=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(commands)
        self.assertFalse(current)


if __name__ == "__main__":
    unittest.main()
