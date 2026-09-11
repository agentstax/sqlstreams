#!/usr/bin/env python3
"""Run the existing lab roles against a native, disposable PostgreSQL database."""
import argparse
import datetime
import json
import hashlib
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time


def run(command, **kwargs):
    return subprocess.check_output(command, text=True, **kwargs).strip()


def interrupted(signum, frame):
    raise KeyboardInterrupt


def main():
    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--postgres-bin', required=True)
    parser.add_argument('--port', type=int, required=True)
    parser.add_argument('--user', required=True)
    parser.add_argument('--scenario', default='max-throughput')
    parser.add_argument('--time-scale', default='1')
    parser.add_argument('--drain-budget', default='2m')
    args = parser.parse_args()
    directory = Path(__file__).resolve().parent
    database = 'reliability_' + datetime.datetime.now().strftime('%Y%m%d_%H%M%S')
    evidence = directory / 'results' / args.scenario / database
    evidence.mkdir(parents=True)
    records = evidence / 'records'
    records.mkdir()
    connection = [str(Path(args.postgres_bin) / 'psql'), '-X', '-h', '127.0.0.1', '-p', str(args.port), '-U', args.user, '-At', '-v', 'ON_ERROR_STOP=1']

    def sql(statement, target='postgres'):
        return run(connection + ['-d', target, '-c', statement])

    data = Path(sql('SHOW data_directory'))
    if sql("SELECT current_setting('fsync') || '|' || current_setting('synchronous_commit') || '|' || current_setting('full_page_writes') || '|' || current_setting('autovacuum')") != 'on|on|on|on':
        raise RuntimeError('durability and autovacuum must remain enabled')
    if sql("SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'client backend' AND pid <> pg_backend_pid()") != '0':
        raise RuntimeError('native server has other clients')
    postgres_pid = int((data / 'postmaster.pid').read_text().splitlines()[0])
    environment = dict(os.environ, POSTGRES_HOST='127.0.0.1', POSTGRES_PORT=str(args.port),
                       POSTGRES_USER=args.user, POSTGRES_DB=database,
                       GOMAXPROCS=os.environ.get('GOMAXPROCS', '4'), GOGC=os.environ.get('GOGC', '400'), GOMEMLIMIT=os.environ.get('GOMEMLIMIT', '2GiB'))
    runtime = {key: environment[key] for key in ('GOMAXPROCS', 'GOGC', 'GOMEMLIMIT')}
    binary_hash = hashlib.sha256()
    with open(args.binary, 'rb') as binary:
        for block in iter(lambda: binary.read(1024 * 1024), b''):
            binary_hash.update(block)
    fingerprint = dict(execution='native', runtime=runtime, binary_sha=binary_hash.hexdigest(),
                       library_sha=run(['git', 'rev-parse', 'HEAD'], cwd=directory),
                       library_dirty=bool(run(['git', 'status', '--porcelain'], cwd=directory)),
                       postgres_image='native',
                       host=dict(os=run(['uname', '-sr']), cpu=run(['sysctl', '-n', 'machdep.cpu.brand_string']),
                                 cores=os.cpu_count(), memory_bytes=int(run(['sysctl', '-n', 'hw.memsize']))))
    (evidence / 'fingerprint.json').write_text(json.dumps(fingerprint, indent=2))
    (evidence / 'source.diff').write_text(run(['git', 'diff', '--', '.bench'], cwd=directory.parent))
    declaration = json.loads(run([args.binary, '-role', 'print-json', '-scenario', args.scenario]))
    (evidence / 'declaration.json').write_text(json.dumps(declaration, indent=2))
    (evidence / 'declaration.txt').write_text(run([args.binary, '-scenario', args.scenario]))
    shutil.copy2(args.binary, evidence / 'reliability')
    shutil.copy2(__file__, evidence / 'native.py')
    for source in directory.rglob('*.go'):
        if 'results' in source.relative_to(directory).parts:
            continue
        destination = evidence / '_source' / source.relative_to(directory)
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)
    common = [str(evidence / 'reliability'), '-scenario', args.scenario, '-time-scale', args.time_scale,
              '-record-dir', str(records)]
    processes = {}
    files = []
    created = False
    previous = {}
    previous_time = time.monotonic()

    def start(role):
        output = open(evidence / (role + '.log'), 'w')
        files.append(output)
        command = common + ['-role', role, '-name', role]
        if role == 'checker':
            command += ['-results-dir', str(directory / 'results'), '-fingerprint-file', str(evidence / 'fingerprint.json'),
                        '-stats-file', str(evidence / 'stats.jsonl'), '-drain-budget', args.drain_budget]
        processes[role] = subprocess.Popen(command, env=environment, stdout=output, stderr=subprocess.STDOUT)

    def sample():
        nonlocal previous, previous_time
        now = time.monotonic()
        at = datetime.datetime.now(datetime.timezone.utc).isoformat()
        entries = {}
        for line in run(['ps', '-axo', 'pid=,ppid=,time=']).splitlines():
            fields = line.split()
            if len(fields) != 3:
                continue
            parts = fields[2].split(':')
            seconds = sum(float(value) * 60 ** index for index, value in enumerate(reversed(parts)))
            entries[int(fields[0])] = (int(fields[1]), seconds)
        with open(evidence / 'stats.jsonl', 'a') as output:
            for role in ('producer', 'consumer', 'postgres'):
                selected = [pid for pid, (parent, _) in entries.items() if role == 'postgres' and (parent == postgres_pid or pid == postgres_pid)]
                if role in processes:
                    selected = [processes[role].pid]
                consumed = sum(max(0, entries[pid][1] - previous[pid]) for pid in selected if pid in entries and pid in previous)
                if selected and previous:
                    output.write(json.dumps(dict(at=at, name=role, service=role, cpu_percent=consumed / (now - previous_time) * 100, cpus=int(runtime['GOMAXPROCS']) if role in ('producer', 'consumer') else 0)) + '\n')
        previous = {pid: value[1] for pid, value in entries.items()}
        previous_time = now
        postgres_bytes, results_bytes = (int(run(['du', '-sk', str(path)]).split()[0]) * 1024 for path in (data, directory / 'results'))
        footprint = postgres_bytes + results_bytes
        free = shutil.disk_usage(data).free
        with open(evidence / 'storage.jsonl', 'a') as output:
            output.write(json.dumps(dict(at=at, footprint_bytes=footprint, postgres_bytes=postgres_bytes, results_bytes=results_bytes, free_bytes=free)) + '\n')
        if footprint > 100_000_000_000 or free < 40 * 1024 ** 3:
            raise RuntimeError('storage guard reached: 100 GB footprint or 40 GiB free floor')
        if declaration.get('DisableExceptionConsumers') and (records / 'producer.phase.jsonl').exists():
            exception_state = json.loads(sql("SELECT json_build_object('enabled', (SELECT count(*) FROM sqlstreams.worker_config WHERE name='exception_consumer' AND target_instances<>0), 'live', (SELECT count(*) FROM sqlstreams.worker_instance i JOIN sqlstreams.worker_config w ON w.id=i.worker_id WHERE w.name='exception_consumer'))", database))
            with open(evidence / 'exception-guard.jsonl', 'a') as output:
                output.write(json.dumps(dict(at=at, **exception_state)) + '\n')
            if exception_state['enabled'] or exception_state['live']:
                raise RuntimeError('exception consumers became enabled or live during the run')
        for role in ('consumer', 'observer'):
            if role in processes and processes[role].poll() is not None:
                raise RuntimeError(role + ' exited before measurement completed')

    def wait(role):
        while processes[role].poll() is None:
            sample()
            time.sleep(1)
        return processes[role].returncode

    def stop(role):
        process = processes.get(role)
        if process is not None and process.poll() is None:
            process.send_signal(signal.SIGTERM)
            try:
                process.wait(timeout=90)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()

    try:
        sample()
        sql('CREATE DATABASE ' + database)
        created = True
        sql('ALTER DATABASE ' + database + " SET deadlock_timeout = '200ms'")
        sql('ALTER DATABASE ' + database + ' SET log_lock_waits = on')
        print(json.dumps(dict(evidence=str(evidence), database=database, runtime=runtime)), flush=True)
        start('consumer')
        start('observer')
        time.sleep(3)
        start('producer')
        if wait('producer') != 0:
            raise RuntimeError('producer failed; inspect producer.log')
        (evidence / 'maintenance-verification.json').write_text(sql("SELECT json_build_object('worker_history_rows_with_failures', (SELECT count(*) FROM sqlstreams.worker_instance_log WHERE attempts>0), 'enabled_exception_consumers', (SELECT count(*) FROM sqlstreams.worker_config WHERE name='exception_consumer' AND target_instances<>0), 'live_exception_consumers', (SELECT count(*) FROM sqlstreams.worker_instance i JOIN sqlstreams.worker_config w ON w.id=i.worker_id WHERE w.name='exception_consumer'))", database))
        (evidence / 'effective-settings.json').write_text(sql("SELECT json_object_agg(name, setting) FROM pg_settings", database))
        (evidence / 'effective-workers.json').write_text(sql("SELECT json_agg(worker_config) FROM sqlstreams.worker_config", database))
        (evidence / 'effective-streams.json').write_text(sql("SELECT json_agg(stream_config) FROM sqlstreams.stream_config", database))
        stop('observer')
        del processes['observer']
        start('checker')
        verdict = wait('checker')
        (evidence / 'exit.json').write_text(json.dumps(dict(checker_exit=verdict)))
        print(json.dumps(dict(evidence=str(evidence), checker_exit=verdict)), flush=True)
        return verdict
    finally:
        for role in list(processes):
            stop(role)
        for output in files:
            output.close()
        if created:
            sql('DROP DATABASE ' + database + ' WITH (FORCE)')
            (evidence / 'cleanup.json').write_text(json.dumps(dict(database=database, dropped=True)))


if __name__ == '__main__':
    raise SystemExit(main())
