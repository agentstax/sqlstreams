"""Run in the user's Terminal; only the selected tracing tool receives administrator access."""
import json, os, pathlib, re, shutil, signal, subprocess, sys, time

def swap_bytes():
    snapshot = subprocess.check_output(['/usr/bin/vm_stat'], text=True)
    pages = sum(int(value) for value in re.findall(r'^Swap(?:ins|outs):\s+(\d+)\.', snapshot, re.M))
    return pages * os.sysconf('SC_PAGE_SIZE')


assert sys.argv[1:] in ([], ['--disk-io'], ['--storage-events']), 'Supported options: --disk-io or --storage-events.'
disk_io = sys.argv[1:] == ['--disk-io']
storage_events = sys.argv[1:] == ['--storage-events']
trace_name = 'storage-events.jsonl' if storage_events else 'disk-io.txt' if disk_io else 'kernel-trace.txt'

assert os.geteuid() != 0, 'Run python normally; the script elevates only tracing.'
assert sys.stdin.isatty(), 'Run this command in Terminal so sudo can authenticate.'
subprocess.run(['sudo', '-v'], check=True)
scratch = pathlib.Path(__file__).resolve().parent
baseline = pathlib.Path('/private/tmp/vulkan-native18.D1I5CE')
control = '/opt/homebrew/opt/postgresql@18/bin/pg_ctl'
name = ('storage_events_' if storage_events else 'storage_diskio_' if disk_io else 'storage_kernel_') + time.strftime('%Y%m%d_%H%M%S', time.gmtime())
directory = scratch / 'results/evidence/native18' / name
roots = [pathlib.Path('/private/tmp/vulkan-page-builds'), baseline,
         pathlib.Path('/private/tmp/vulkan-native.ULMBSf'),
         pathlib.Path('/private/tmp/vulkan-smartmontools'), scratch / 'results']
retained = sum(int(subprocess.check_output(['du', '-sk', str(root)], text=True).split()[0]) * 1024 for root in roots)
assert retained + 16 * 1024**3 < 100 * 1000**3, '100 GB benchmark storage budget exceeded.'
assert shutil.disk_usage(baseline).free > 60 * 1024**3, 'Insufficient free-space reserve.'
command = [sys.executable, str(scratch / 'storage_isolate.py'), '--directory', str(directory),
           '--seconds', '180', '--mixed', '--wal-gib', '8', '--wal-prefill',
           '--wal-rate-mib', '250', '--data-gib', '8', '--data-block-kib', '1024',
           '--data-rate-mib', '600', '--aligned-buffer', '--no-catchup']
subprocess.run([control, '-D', str(baseline / 'pgdata'), 'stop', '-m', 'fast', '-w'], check=True)
workload = trace = report = None
trace_swap_start = trace_swap_end = None
swap_history = []
try:
    print('Checking swap activity for ten seconds before starting.', flush=True)
    before = swap_bytes()
    time.sleep(10)
    assert swap_bytes() - before <= 16 * 1024**2, 'Memory is actively swapping; wait for other large jobs to finish and retry.'
    workload = subprocess.Popen(command, start_new_session=True)
    print('Watching two scratch writers for a write over 100 ms. Maximum run: three minutes.', flush=True)
    while workload.poll() is None:
        now = time.monotonic()
        swaps = swap_bytes()
        swap_history.append((now, swaps))
        while len(swap_history) > 1 and now - swap_history[1][0] >= 10:
            swap_history.pop(0)
        quiet = now - swap_history[0][0] >= 10 and swaps - swap_history[0][1] <= 16 * 1024**2
        if directory.exists():
            with (directory / 'swap-guard.jsonl').open('a') as output:
                output.write(json.dumps(dict(at=time.time(), swap_bytes=swaps, quiet=quiet)) + '\n')
        if trace is not None and trace_swap_end is None and trace.poll() is not None:
            trace_swap_end = swaps
        if trace is None:
            writers, slow = [], False
            for role in ['wal', 'data']:
                path = directory / (role + '.jsonl')
                if not path.exists():
                    continue
                rows = []
                for line in path.read_text().splitlines():
                    try:
                        rows.append(json.loads(line))
                    except json.JSONDecodeError:
                        pass  # The writer may be midway through its current line.
                configs = [row for row in rows if row.get('kind') == 'config']
                samples = [row for row in rows if row.get('kind') == 'sample']
                if configs:
                    writers.append(configs[0]['pid'])
                if samples and time.time() - samples[-1]['at'] < 3 and samples[-1]['maximum_ms'] > 100:
                    slow = True
            if len(writers) == 2 and slow and quiet:
                capture = ['sudo', '-n', '/usr/sbin/spindump', str(writers[0]), '10', '10',
                           '-onlyTarget', '-proc', str(writers[1]), '-proc', '0', '-noBinary', '-noFile']
                if disk_io:
                    capture = ['sudo', '-n', '/usr/bin/fs_usage', '-w', '-f', 'diskio', '-t', '20']
                if storage_events:
                    # Installed kdebug.h: filesystem disk I/O, IOKit storage, storage drivers.
                    capture = ['sudo', '-n', '/usr/bin/ktrace', 'trace', '-T', '20', '-b', '64',
                               '-f', 'S0x0302,S0x0520,S0x0601', '--ndjson']
                (directory / 'trace-command.json').write_text(json.dumps(dict(at=time.time(), command=capture)))
                trace_swap_start = swaps
                (directory / 'processes-at-trace.txt').write_text(subprocess.check_output(['ps', '-axo', 'pid,ppid,rss,comm'], text=True))
                report = (directory / trace_name).open('w')
                trace = subprocess.Popen(capture, stdout=report, stderr=subprocess.STDOUT)
                print('Stall detected; capturing filesystem and storage-driver events for twenty seconds.' if storage_events else 'Stall detected without heavy swapping; capturing disk requests for twenty seconds.' if disk_io else 'Stall detected without heavy swapping; capturing writers and kernel_task for ten seconds.', flush=True)
        time.sleep(1)
    assert workload.returncode == 0, 'Scratch workload did not complete successfully.'
    if trace is not None:
        assert trace.wait(timeout=60) == 0, 'Administrator trace failed; inspect ' + trace_name
        if trace_swap_end is None:
            trace_swap_end = swap_bytes()
        (directory / 'trace-memory.json').write_text(json.dumps(dict(swap_bytes=trace_swap_end-trace_swap_start, usable_without_heavy_swapping=trace_swap_end-trace_swap_start <= 64*1024**2)))
    else:
        print('No qualifying stall during quiet swap conditions; no trace captured.', flush=True)
finally:
    try:
        if workload is not None and workload.poll() is None:
            os.killpg(workload.pid, signal.SIGINT)
            try:
                workload.wait(timeout=30)
            except subprocess.TimeoutExpired:
                os.killpg(workload.pid, signal.SIGKILL)
                workload.wait()
        if trace is not None and trace.poll() is None:
            trace.wait(timeout=60)
    finally:
        if report is not None:
            report.close()
        for role in ['wal', 'data']:
            (directory / (role + '.bin')).unlink(missing_ok=True)
        subprocess.run([control, '-D', str(baseline / 'pgdata'), '-l', str(baseline / 'postgres.log'), 'start', '-w'], check=True)

subprocess.run([sys.executable, str(scratch / 'storage_analyze.py'), str(directory)], check=True)
record = dict(run=name, study='storage_driver_events' if storage_events else 'disk_request_trace' if disk_io else 'kernel_trace', command=command, trace_captured=trace is not None,
              trace_memory=json.loads((directory / 'trace-memory.json').read_text()) if trace else None,
              evidence=str(directory), result=json.loads((directory / 'analysis.json').read_text()))
(directory / 'capture_kernel.py').write_bytes(pathlib.Path(__file__).read_bytes())
with (scratch / 'results/runs.jsonl').open('a') as output:
    output.write(json.dumps(record) + '\n')
print('Done. PostgreSQL restored; evidence: ' + str(directory), flush=True)
