"""Summarize storage controls; host counters cover accessible live processes only."""
import json, pathlib, sys

directory = pathlib.Path(sys.argv[1])
result = dict(run=directory.name, roles={})
for role in ['wal', 'data']:
    path = directory/(role+'.jsonl')
    if not path.exists(): continue
    rows = [json.loads(line) for line in path.read_text().splitlines()]
    final = next(row for row in rows if row['kind']=='final')
    samples = [row for row in rows if row['kind']=='sample']
    result['roles'][role] = dict(mib_s=final['bytes']/final['elapsed_s']/1024**2, maximum_sampled_write_ms=max(row['maximum_ms'] for row in samples), intervals_over_20ms=sum(row['maximum_ms']>20 for row in samples), windows=[dict(end_s=row['elapsed_s'],mib_s=row['interval_bytes']/row['interval_s']/1024**2,maximum_ms=row['maximum_ms'],fsync_ms=row['fsync_ms']) for row in samples])
rows = [json.loads(line) for line in (directory/'host.jsonl').read_text().splitlines()]
totals = {}; previous = {}
for row in rows:
    current = {(process['pid'],process['start']):process for process in row['processes']}
    for identity, process in current.items():
        if identity not in previous: continue
        total = totals.setdefault(identity,dict(pid=process['pid'],command=process['command'],read_bytes=0,write_bytes=0))
        for field in ['read_bytes','write_bytes']:
            total[field] += max(0,process[field]-previous[identity][field])
    previous = current
result['top_process_io'] = sorted(totals.values(),key=lambda row:row['write_bytes']+row['read_bytes'],reverse=True)[:15]
result['unavailable_processes_max'] = max(row['unavailable_processes'] for row in rows)
result['minimum_free_bytes'] = min(row['free_bytes'] for row in rows)
if (directory/'smart.jsonl').exists():
    health = [json.loads(line) for line in (directory/'smart.jsonl').read_text().splitlines()]
    writes = [json.loads(line) for line in (directory/'wal.jsonl').read_text().splitlines()]
    config = next(row for row in writes if row['kind']=='config')
    started = config['at']
    windows = []
    for low in range(0, int(rows[-1]['at']-started), 30):
        high = low+30
        before = min(rows,key=lambda row:abs(row['at']-started-low))
        after = min(rows,key=lambda row:abs(row['at']-started-high))
        span = after['at']-before['at']
        if span<=0:continue
        old = next(item['statistics'] for item in before['device_io'] if item['name']=='IOBlockStorageDriver')
        new = next(item['statistics'] for item in after['device_io'] if item['name']=='IOBlockStorageDriver')
        operations = new['Operations (Write)']-old['Operations (Write)']
        temperatures = [row['result']['nvme_smart_health_information_log']['temperature'] for row in health if started+low<=row['at']<started+high and row['returncode']==0]
        window = dict(start_s=low,end_s=high,observed_start_s=before['at']-started,observed_end_s=after['at']-started,driver_write_mb_s=(new['Bytes (Write)']-old['Bytes (Write)'])/span/1e6,driver_write_iops=operations/span,driver_write_ms=(new['Total Time (Write)']-old['Total Time (Write)'])/operations/1e6 if operations else None,temperature_min_c=min(temperatures) if temperatures else None,temperature_max_c=max(temperatures) if temperatures else None)
        for role in result['roles']:
            samples = [row for row in result['roles'][role]['windows'] if low<row['end_s']<=high]
            window[role+'_maximum_ms'] = max((row['maximum_ms'] for row in samples),default=None)
        windows.append(window)
    result['health_windows'] = windows
    result['health_limitations'] = 'SMART composite temperature only; unavailable controller thermal and internal NAND counters prevent ruling out every internal thermal or firmware mechanism. Driver timing includes queueing, alignment and transfer, excludes separate flush operations. Windows use nearest observed endpoints; pause boundaries can straddle intervals.'
if (directory/'data.jsonl').exists():
    data = [json.loads(line) for line in (directory/'data.jsonl').read_text().splitlines()]
    config = next(row for row in data if row['kind']=='config')
    data_samples = [row for row in data if row['kind'] in ('sample','final')]
    phases = []
    for low, high in [(1,24),(30,44)]:
        first = min(data_samples,key=lambda row:abs(row['elapsed_s']-low))
        last = min(data_samples,key=lambda row:abs(row['elapsed_s']-high))
        if last['elapsed_s']-first['elapsed_s']<5:continue
        snapshots = [(row,process) for row in rows for process in row['processes'] if process['pid']==config['pid']]
        before, old = min(snapshots,key=lambda item:abs(item[0]['at']-config['at']-low))
        after, new = min(snapshots,key=lambda item:abs(item[0]['at']-config['at']-high))
        phases.append(dict(start_s=low,end_s=high,submitted_mib_s=(last['bytes']-first['bytes'])/(last['elapsed_s']-first['elapsed_s'])/1024**2,host_read_bytes=new['read_bytes']-old['read_bytes'],host_write_bytes=new['write_bytes']-old['write_bytes'],host_span_s=after['at']-before['at']))
    result['data_phases'] = phases
result['limitations'] = 'Per-process counters omit inaccessible processes and activity between final observation and process exit. Sample latency maxima omit the last partial interval. Buffered writes, file reuse, and fsync cadence approximate components, not a complete PostgreSQL workload.'
(directory/'analysis.json').write_text(json.dumps(result))
print(json.dumps({key:value for key,value in result.items() if key!='roles'}))
for role, values in result['roles'].items(): print(role,{key:value for key,value in values.items() if key!='windows'})
