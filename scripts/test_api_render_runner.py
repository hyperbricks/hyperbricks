"""API runner ownership/preflight checks without Docker or real network listeners."""

import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('run_api_fragment_render_tests_docker.sh')


class APIRenderRunnerTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.tmp = self.root / 'tmp'
        self.tmp.mkdir()
        (self.root / 'scripts').mkdir()
        shutil.copy2(SCRIPT, self.root / 'scripts' / SCRIPT.name)
        self.events_file = self.root / 'events'
        self.projects_file = self.root / 'docker-projects'
        self.pid_file = self.root / 'helper.pid'
        self.lock = self.tmp / f'hyperbricks-api-render-tests-{os.getuid()}.lock'
        self.environment = dict(os.environ, PATH=str(self.bin), TMPDIR=str(self.tmp),
                                EVENTS=str(self.events_file), HELPER_PID_FILE=str(self.pid_file),
                                DOCKER_PROJECTS=str(self.projects_file),
                                TEST_SERVER_LOG=str(self.root / 'helper.log'), POSTGREST_PORT='13001',
                                POSTGRES_PASSWORD='fixture-password', PGRST_JWT_SECRET='fixture-jwt',
                                PGADMIN_DEFAULT_PASSWORD='fixture-admin')
        for name in ('dirname', 'mktemp', 'mkdir', 'rmdir', 'rm', 'sleep', 'seq', 'tail',
                     'awk', 'openssl', 'cat', 'head', 'id'):
            source = shutil.which(name)
            if source:
                (self.bin / name).symlink_to(source)
        self.sentinel = subprocess.Popen(['/bin/sleep', '60'])
        self.addCleanup(self.stop_sentinel)
        self.environment['SENTINEL_PID'] = str(self.sentinel.pid)
        self.addCleanup(self.stop_orphan_helper)
        self.stub('docker', '''
import os, sys
args = sys.argv[1:]
if 'compose' in args and 'version' in args:
    sys.exit(1 if os.environ.get('NO_COMPOSE') else 0)
if args and args[0] == 'info':
    sys.exit(1 if os.environ.get('NO_DAEMON') else 0)
for action in ('up', 'down'):
    if action in args:
        event('docker ' + action)
        project = args[args.index('-p') + 1] if '-p' in args else '<default>'
        with open(os.environ['DOCKER_PROJECTS'], 'a') as log: log.write(action + ' ' + project + '\\n')
        sys.exit(1 if action == 'up' and os.environ.get('FAIL_UP') else 0)
sys.exit(0)
''')
        self.stub('lsof', '''
import os, sys
port = os.environ.get('BUSY_PORT')
if port and any('TCP:' + port in arg for arg in sys.argv[1:]):
    if any(arg == '-t' or arg.startswith('-t') for arg in sys.argv[1:]):
        print(os.environ['SENTINEL_PID'])
    else:
        print('COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME')
        print('main ' + os.environ['SENTINEL_PID'] + ' user 1u IPv4 0 0 TCP *:' + port + ' (LISTEN)')
    sys.exit(0)
sys.exit(1)
''')
        self.stub('go', '''
import os, pathlib, sys
args = sys.argv[1:]
if args and args[0] == 'build':
    event('go build')
    if os.environ.get('FAIL_BUILD'): sys.exit(17)
    target = pathlib.Path(args[args.index('-o') + 1])
    if not str(target.resolve()).startswith(str(pathlib.Path(os.environ['TMPDIR']).resolve()) + os.sep):
        sys.exit('Helper build must use a private temporary path')
    target.parent.mkdir(parents=True, exist_ok=True)
    helper = """import os, pathlib, signal, time
pathlib.Path(os.environ['HELPER_PID_FILE']).write_text(str(os.getpid()))
def stop(signum, frame):
    with open(os.environ['EVENTS'], 'a') as log: log.write('helper stopped\\\\n')
    raise SystemExit(0)
signal.signal(signal.SIGTERM, stop)
while True: time.sleep(0.02)
"""
    target.write_text('#!' + sys.executable + '\\n' + helper)
    target.chmod(0o755)
    sys.exit(0)
if args and args[0] == 'test':
    event('go test')
    sys.exit(13 if os.environ.get('FAIL_TESTS') else 0)
sys.exit('Unexpected go invocation: ' + repr(args))
''')
        self.stub('curl', '''
import os, pathlib, sys, time
if any('8090' in arg for arg in sys.argv[1:]):
    for attempt in range(100):
        if pathlib.Path(os.environ['HELPER_PID_FILE']).exists(): sys.exit(0)
        time.sleep(0.01)
    sys.exit(1)
sys.exit(0)
''')

    def stub(self, name, body):
        path = self.bin / name
        path.write_text('#!' + sys.executable + '\n'
                        'import os\n'
                        'def event(value):\n'
                        '    with open(os.environ["EVENTS"], "a") as log: log.write(value + "\\n")\n'
                        + body)
        path.chmod(0o755)

    def stop_sentinel(self):
        if self.sentinel.poll() is None:
            self.sentinel.terminate()
        self.sentinel.wait(timeout=3)

    def stop_orphan_helper(self):
        if self.pid_file.exists():
            try:
                os.kill(int(self.pid_file.read_text()), signal.SIGTERM)
            except ProcessLookupError:
                pass

    def events(self):
        return self.events_file.read_text().splitlines() if self.events_file.exists() else []

    def run_runner(self, success=False):
        result = subprocess.run(['/bin/bash', str(self.root / 'scripts' / SCRIPT.name)],
                                cwd=self.root, env=self.environment, capture_output=True,
                                text=True, timeout=10)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def assert_preflight_left_other_processes_alone(self):
        self.assertIsNone(self.sentinel.poll(), 'Unrelated listener was killed')
        self.assertEqual(self.events(), [], 'Blocked runner touched helpers or Docker stack')
        self.assertFalse(self.pid_file.exists())

    def test_missing_docker_cli_fails_without_starting_or_stopping_anything(self):
        (self.bin / 'docker').unlink()
        result = self.run_runner()
        self.assertIn('docker', (result.stdout + result.stderr).lower())
        self.assert_preflight_left_other_processes_alone()

    def test_unavailable_compose_or_daemon_fails_without_stack_cleanup(self):
        for variable in ('NO_COMPOSE', 'NO_DAEMON'):
            with self.subTest(variable=variable):
                self.environment[variable] = '1'
                self.run_runner()
                self.environment.pop(variable)
                self.assert_preflight_left_other_processes_alone()

    def test_busy_helper_or_postgrest_port_preserves_even_a_process_named_main(self):
        for port in ('8090', self.environment['POSTGREST_PORT']):
            with self.subTest(port=port):
                self.environment['BUSY_PORT'] = port
                self.run_runner()
                self.assert_preflight_left_other_processes_alone()
                self.assertFalse(self.lock.exists(), 'Rejected run leaked its lock')

    def test_existing_lock_is_preserved_without_touching_the_other_stack(self):
        self.lock.mkdir()
        owner = self.lock / 'owner'
        owner.write_text('another active runner\n')
        self.run_runner()
        self.assert_preflight_left_other_processes_alone()
        self.assertEqual(owner.read_text(), 'another active runner\n')

    def test_default_postgrest_port_3000_is_checked(self):
        self.environment.pop('POSTGREST_PORT')
        self.environment['BUSY_PORT'] = '3000'
        self.run_runner()
        self.assert_preflight_left_other_processes_alone()
        self.assertFalse(self.lock.exists())

    def test_build_failure_releases_lock_without_stopping_an_unstarted_stack(self):
        self.environment['FAIL_BUILD'] = '1'
        self.run_runner()
        self.assertIsNone(self.sentinel.poll())
        self.assertEqual(self.events(), ['go build'])
        self.assertFalse(self.pid_file.exists())
        self.assertFalse(self.lock.exists())

    def test_success_stops_only_its_helper_and_stack_and_removes_its_lock(self):
        self.run_runner(success=True)
        self.assert_owned_cleanup()
        self.assertEqual(self.events(), ['go build', 'docker up', 'go test', 'docker down', 'helper stopped'])
        first_project = self.projects_file.read_text().splitlines()[0].split()[1]
        for path in (self.events_file, self.projects_file, self.pid_file):
            path.unlink()
        self.run_runner(success=True)
        self.assert_owned_cleanup()
        second_project = self.projects_file.read_text().splitlines()[0].split()[1]
        self.assertNotEqual(first_project, second_project)

    def test_failed_tests_still_clean_up_only_owned_resources(self):
        self.environment['FAIL_TESTS'] = '1'
        self.run_runner()
        self.assert_owned_cleanup()
        self.assertIn('go test', self.events())

    def test_failed_stack_start_cleans_owned_partial_stack_and_helper(self):
        self.environment['FAIL_UP'] = '1'
        self.run_runner()
        self.assert_owned_cleanup()
        self.assertNotIn('go test', self.events())

    def assert_owned_cleanup(self):
        self.assertIsNone(self.sentinel.poll(), 'Runner stopped an unrelated process')
        self.assertTrue(self.pid_file.exists(), 'Fixture helper did not run')
        with self.assertRaises(ProcessLookupError):
            os.kill(int(self.pid_file.read_text()), 0)
        self.assertEqual(self.events().count('docker up'), 1)
        self.assertEqual(self.events().count('docker down'), 1)
        self.assertEqual(self.events().count('helper stopped'), 1)
        actions = [line.split() for line in self.projects_file.read_text().splitlines()]
        self.assertEqual([action for action, project in actions], ['up', 'down'])
        self.assertEqual(actions[0][1], actions[1][1], 'Cleanup targeted a different Compose project')
        self.assertRegex(actions[0][1], r'^hyperbricks-api-tests-[0-9]+$')
        self.assertFalse(self.lock.exists(), 'Runner left its own lock behind')


if __name__ == '__main__':
    unittest.main()
