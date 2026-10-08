#!/usr/bin/env python3
"""Configuration profile contracts through the real CLI, HTTP, and archives."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import zipfile

from test_project_lifecycle import ROOT, MODULE, NAME, staging, check, run, server, free_port, html


def verify(binary, workspace):
    project = workspace / 'configuration'
    module = staging.stage(MODULE, project)
    entry = module / 'package.hyperbricks.yaml'
    entry.write_bytes((module / 'package.configuration.hyperbricks.yaml').read_bytes())
    events = module / 'lifecycle-events.jsonl'
    css = module / 'resources/css/site.css'
    original_css = css.read_text()
    assets = module / 'static/css'
    assets.mkdir(parents=True, exist_ok=True)
    sentinel = assets / 'unowned.css'
    sentinel.write_text('/* user-owned */')

    def records():
        return [json.loads(line) for line in events.read_text().splitlines()] if events.exists() else []

    def phases(expected, outcome, failed=''):
        rows = records()
        operation = 'start' if expected[0] == 'before_start' else 'static'
        check(all(row['operation'] == operation for row in rows), f'Wrong operation: {rows}')
        check([row['hook_phase'] for row in rows] == expected, f'Wrong hook order: {rows}')
        check(rows[-1]['outcome'] == outcome and rows[-1]['failed_phase'] == failed,
              f'Wrong finish context: {rows[-1]}')
        check(rows[-1]['exit_code'] == ('1' if outcome == 'failure' else '0'), f'Wrong exit: {rows[-1]}')
        events.unlink()

    def bundles():
        return {p.name: p.read_bytes() for p in assets.glob('site*.css')}

    def export(label, enabled=True, success=True):
        args = [binary, 'static', '-m', NAME, '--force', '--zip', '--non-interactive']
        if enabled:
            args.append('--with-processes')
        log = workspace / (label + '.log')
        with log.open('w') as output:
            result = subprocess.run(list(map(str, args)), cwd=project, stdout=output,
                                    stderr=subprocess.STDOUT, timeout=90)
        check((result.returncode == 0) == success, f'Unexpected export exit; see {log}')

    export('disabled', enabled=False)
    check(not records(), 'Hooks executed without opt-in')
    first = bundles()
    check(len(first) == 1, f'Expected one fingerprinted bundle: {first.keys()}')
    export('success')
    phases(['before_static', 'after_static', 'finish'], 'success')
    check(bundles() == first, 'Cache hit changed bundle bytes or names')
    css.write_text(original_css + '\n.configuration-proof { color: #123456; }\n')
    export('changed')
    phases(['before_static', 'after_static', 'finish'], 'success')
    second = bundles()
    check(len(second) == 1 and second.keys() != first.keys(), 'Obsolete fingerprint was retained')
    archives = list((project / 'exports' / NAME).glob('*.zip'))
    check(archives, 'ZIP missing')
    with zipfile.ZipFile(max(archives, key=lambda p: p.stat().st_mtime_ns)) as archive:
        names = {Path(name).name for name in archive.namelist()}
        check(set(second) <= names and not (set(first) & names), 'ZIP contains stale or missing assets')
    check(sentinel.read_text() == '/* user-owned */', 'Cleanup removed unrelated assets')
    css.write_text('@import "./missing-configuration-fixture.css";\n')
    export('failed', success=False)
    phases(['before_static', 'finish'], 'failure', 'render')
    check(bundles() == second, 'Failed build removed working assets')
    css.write_text(original_css)
    print('PASS static opt-in, hook order/outcomes, ZIP timing, cache hit, cleanup, failure preservation', flush=True)

    # Each session resolves the imported variable after entry overrides. The
    # second session also changes CSS, proving cleanup at a real restart.
    for iteration in range(2):
        port = free_port()
        entry.write_text(entry.read_text().replace('fixture_port: 8107', f'fixture_port: {port}'))
        if iteration:
            css.write_text(original_css + '\n.restart-proof { color: #654321; }\n')
        base = f'http://127.0.0.1:{port}'
        with server([binary, 'start', '-m', NAME, '--with-processes', '--non-interactive'],
                    project, workspace / f'start-{iteration}.log', base + '/'):
            html(base, '/')
            for _ in range(50):
                if len(records()) >= 2:
                    break
                time.sleep(.1)
            check([r['hook_phase'] for r in records()] == ['before_start', 'after_start'], 'Startup order wrong')
            check(all(r['server_port'] == str(port) for r in records()), 'Imported variable override not used')
        phases(['before_start', 'after_start', 'finish'], 'cancelled')
        check(len(bundles()) == 1, 'Restart retained obsolete fingerprints')
        entry.write_text(entry.read_text().replace(f'fixture_port: {port}', 'fixture_port: 8107'))
    check(sentinel.exists(), 'Restart removed unrelated file')
    print('PASS imported override, start/restart hook order, cancellation finish, native startup cleanup', flush=True)

    # Package imports must survive an archive build, not only local execution.
    run([binary, 'build', '--hra', '-m', NAME, '--non-interactive'], project, workspace / 'archive.log')
    archive = next((project / 'deploy' / NAME).glob('*.hra'))
    with zipfile.ZipFile(archive) as z:
        names = z.namelist()
        for suffix in ('config/lifecycle.hyperbricks.yaml', 'package.static.hyperbricks.yaml'):
            check(any(name.endswith(suffix) for name in names), f'Archive lost import {suffix}')
    check(not records(), 'Build executed lifecycle hooks')
    fragment = module / 'config/lifecycle.hyperbricks.yaml'
    fragment.rename(fragment.with_suffix('.moved'))
    export('missing-import', success=False)
    check(not records(), 'Invalid import graph executed hooks')
    print('PASS archive import preservation and missing-import rejection before hooks', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path)
    parser.add_argument('--keep', action='store_true')
    args = parser.parse_args()
    workspace = Path(tempfile.mkdtemp(prefix='hb-configuration-'))
    print(f'Verification workspace: {workspace}', flush=True)
    try:
        binary = args.binary.resolve() if args.binary else workspace / 'hyperbricks'
        if not args.binary:
            run(['go', 'build', '-o', binary, './cmd/hyperbricks'], ROOT, workspace / 'compile.log')
        verify(binary, workspace)
    except Exception:
        print(f'FAILED; retained diagnostics at {workspace}', flush=True)
        raise
    if not args.keep:
        shutil.rmtree(workspace)
    print('All configuration lifecycle checks passed.', flush=True)


if __name__ == '__main__':
    main()
