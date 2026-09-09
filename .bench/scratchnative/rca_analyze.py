import collections, datetime, json, pathlib, re, statistics, sys

directory = pathlib.Path(sys.argv[1])
def timestamp(value):
    return datetime.datetime.fromisoformat(re.sub(r'\.(\d+)', lambda match: '.'+match[1][:6].ljust(6, '0'), value.replace('Z', '+00:00'))).timestamp()
def rows(name):
    return [json.loads(line) for line in (directory/name).read_text().splitlines()]

producer = rows('producer.jsonl')
samples = [row for row in producer if row['kind']=='sample']
final = next(row for row in producer if row['kind']=='final')
started = timestamp(final['at'])-final['elapsed_s']
server = [json.loads((directory/'server-before.json').read_text())]
server += [row['server_stats'] for row in rows('monitor.jsonl')]
server.append(json.loads((directory/'server-after.json').read_text()))
waits = []
for line in (directory/'waits.jsonl').read_text().splitlines():
    try:
        row = json.loads(line)
        if 'sessions' in row: waits.append(row)
    except ValueError:
        pass
disk = []
disk_started = json.loads((directory/'iostat-start.json').read_text())['at']
for line in (directory/'iostat.txt').read_text().splitlines():
    fields = line.split()
    try:
        if len(fields)==9: disk.append([float(value) for value in fields])
    except ValueError:
        pass
windows = []
for low in range(0, int(final['elapsed_s']), 10):
    high = min(low+10, final['elapsed_s'])
    if high-low<5: continue
    before = min(server, key=lambda row: abs(timestamp(row['at'])-started-low))
    after = min(server, key=lambda row: abs(timestamp(row['at'])-started-high))
    elapsed = timestamp(after['at'])-timestamp(before['at'])
    first = min(samples, key=lambda row: abs(row['elapsed_s']-low)) if low else dict(completed=0, elapsed_s=0)
    last = min(samples+[final], key=lambda row: abs(row['elapsed_s']-high))
    counts = collections.Counter()
    frames = [row for row in waits if low<=timestamp(row['at'])-started<high]
    for frame in frames:
        counts.update((row['wait_event_type'],row['wait_event']) for row in frame['sessions'] or [] if row['application_name']=='scratch-producer' and row['state']=='active')
    interval = dict(start_s=low,end_s=high,rate=(last['completed']-first['completed'])/(last['elapsed_s']-first['elapsed_s']),server_span_s=elapsed,wal_wait_sessions=counts[('LWLock','WALWrite')]/len(frames),buffer_wait_sessions=counts[('LWLock','BufferContent')]/len(frames),wal_init_wait_sessions=sum(value for key,value in counts.items() if key[1] in ('WalInitSync','WalInitWrite'))/len(frames))
    for label, backend, target, context in [('wal','client backend','wal','normal'),('relation','client backend','relation','normal'),('checkpoint','checkpointer','relation','normal'),('wal_init','client backend','wal','init')]:
        first_io = next(row for row in before['io'] if row['backend_type']==backend and row['object']==target and row['context']==context)
        last_io = next(row for row in after['io'] if row['backend_type']==backend and row['object']==target and row['context']==context)
        interval[label] = {key:last_io[key]-first_io[key] for key in ('reads','read_bytes','read_time','writes','write_bytes','write_time','fsyncs','fsync_time','extends','extend_time') if first_io.get(key) is not None}
    disk_samples = [row for index,row in enumerate(disk) if index>0 and low<=disk_started+index-started<high]
    if disk_samples:
        interval['disk_MB_s'] = statistics.mean(row[2] for row in disk_samples)
        interval['disk_transfers_s'] = statistics.mean(row[1] for row in disk_samples)
        interval['host_cpu_busy_percent'] = statistics.mean(100-row[5] for row in disk_samples)
    windows.append(interval)
result = dict(run=directory.name,rate=final['completed']/final['elapsed_s'],windows=windows,notes='PostgreSQL cumulative counter publication can lag ongoing work, especially checkpoints; window deltas are publication intervals, not exact operation timing. iostat includes all host disk activity and excludes its since-boot row.')
(directory/'rca-analysis.json').write_text(json.dumps(result))
for row in windows:
    wal = row['wal']
    print(f"{row['start_s']:2}-{row['end_s']:2.0f}s {row['rate']/1000:6.1f}k/s WAL {wal['write_time']/max(1,wal['writes']):5.2f}ms/write waits {row['wal_wait_sessions']:4.2f} buffers {row['buffer_wait_sessions']:4.2f} disk {row.get('disk_MB_s',0):5.0f}MB/s checkpoint {row['checkpoint']['write_bytes']/1e6:5.0f}MB")
