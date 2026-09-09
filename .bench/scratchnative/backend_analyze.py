"""Align backend and host I/O deltas; observations cover live processes only."""
import collections, datetime, json, pathlib, re, sys

directory = pathlib.Path(sys.argv[1])
def timestamp(value):
    return datetime.datetime.fromisoformat(re.sub(r'\.(\d+)', lambda match: '.'+match[1][:6].ljust(6,'0'), value.replace('Z','+00:00'))).timestamp()
producer = [json.loads(line) for line in (directory/'producer.jsonl').read_text().splitlines()]
final = next(row for row in producer if row['kind']=='final')
started = timestamp(final['at'])-final['elapsed_s']
rows = [json.loads(line) for line in (directory/'monitor.jsonl').read_text().splitlines()]
windows = {}
for before, after in zip(rows,rows[1:]):
    elapsed = after['at']-started
    window = windows.setdefault(int(elapsed//10)*10, dict(host={},postgres={},indexes={}))
    identities = {backend['pid']:backend['backend_type']+':'+backend['application_name'] for row in [before,after] for backend in row['backend_io'] or []}
    previous = {(process['pid'],process['start']):process for process in before['host_io']}
    for process in after['host_io']:
        old = previous.get((process['pid'],process['start']))
        if old is None:continue
        label = identities.get(process['pid'],process['command'])
        total = window['host'].setdefault(label,collections.Counter())
        for field in ['read_bytes','write_bytes','user_ns','system_ns']:
            total[field] += max(0,process[field]-old[field])
    previous = {(backend['pid'],backend['backend_start']):backend for backend in before['backend_io'] or []}
    for backend in after['backend_io'] or []:
        old = previous.get((backend['pid'],backend['backend_start']))
        if old is None:continue
        for item in backend['io'] or []:
            earlier = next((entry for entry in old['io'] or [] if (entry['object'],entry['context'])==(item['object'],item['context'])),None)
            if earlier is None:continue
            label = backend['backend_type']+':'+backend['application_name']+':'+item['object']+':'+item['context']
            total = window['postgres'].setdefault(label,collections.Counter())
            for field in ['reads','read_bytes','read_time','writes','write_bytes','write_time','extends','extend_time','fsyncs','fsync_time','evictions']:
                if item.get(field) is not None:total[field]+=max(0,item[field]-earlier[field])
    previous = {item['indexrelid']:item for item in before['index_io'] or []}
    for item in after['index_io'] or []:
        if item['indexrelid'] in previous:
            count = item['idx_blks_read']-previous[item['indexrelid']]['idx_blks_read']
            window['indexes'][item['indexrelname']] = window['indexes'].get(item['indexrelname'],0)+count
result = dict(run=directory.name,windows=windows,notes='Deltas assigned to ten-second windows by their ending sample; first/last partial intervals and process exits may be omitted. PostgreSQL statistics can lag. macOS counters include filesystem I/O not necessarily explicit PostgreSQL reads.',unavailable_max=max(row['host_io_unavailable'] for row in rows))
(directory/'backend-analysis.json').write_text(json.dumps(result))
for start,window in windows.items():
    print(start,'host MB read/write',[(label,round(values['read_bytes']/1e6),round(values['write_bytes']/1e6)) for label,values in window['host'].items() if values['read_bytes']+values['write_bytes']>1e8])
    print(' PG',[(label,dict(values)) for label,values in window['postgres'].items() if values['read_bytes']+values['write_bytes']>1e7 or values['extend_time']>100])
    print(' index read blocks',[(name,count) for name,count in window['indexes'].items() if count])
