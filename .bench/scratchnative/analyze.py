from pathlib import Path
import json,collections,statistics,sys,datetime,re
p=Path(sys.argv[1]);print(p)
producer_rows=[json.loads(x) for x in (p/'producer.jsonl').read_text().splitlines()]
producer_end=next((r.get('at') for r in reversed(producer_rows) if r['kind']=='final'),None)
def timestamp(t):return datetime.datetime.fromisoformat(re.sub(r'\.(\d+)',lambda m:'.'+m[1][:6].ljust(6,'0'),t.replace('Z','+00:00'))).timestamp()
producer_samples=[r for r in producer_rows if r['kind']=='sample']
production_start=timestamp(producer_samples[0]['at'])-producer_samples[0]['elapsed_s'] if producer_samples else 0

for role in [path.stem for pattern in ('producer*.jsonl','consumer*.jsonl') for path in sorted(p.glob(pattern))]:
 rows=[json.loads(x) for x in (p/(role+'.jsonl')).read_text().splitlines()]
 final=next((r for r in reversed(rows) if r['kind']=='final'),None)
 samples=[r for r in rows if r['kind']=='sample' and (not producer_end or timestamp(r['at'])<=timestamp(producer_end)) and timestamp(r['at'])>=production_start]
 print(role,'final',final)
 if len(samples)>6:
  start=samples[5];end=samples[-1]
  print(' steady rate',round((end['completed']-start['completed'])/(end['elapsed_s']-start['elapsed_s'])),'heap peak MB',round(max(r['heap_bytes'] for r in samples)/1e6,1),'pool acquired median',statistics.median(r['pool_acquired'] for r in samples[5:]),'pool wait sec',round((end.get('pool_wait_ns',0)-start.get('pool_wait_ns',0))/1e9,4),'GC CPU fraction',end.get('gc_cpu_fraction'))
frames=[]
for line in (p/'waits.jsonl').read_text().splitlines():
 try:
  r=json.loads(line)
  if 'sessions' in r:frames.append(r)
 except json.JSONDecodeError:pass
frames=frames[50:]
print('wait frames after warmup',len(frames))
for role in ('scratch-producer','scratch-consumer'):
 counts=collections.Counter();active=[];running=[]
 for f in frames:
  sessions=[r for r in f['sessions'] or [] if r['application_name']==role and r['state']!='idle']
  active.append(len(sessions));running.append(sum(r['state']=='active' and r['wait_event'] is None for r in sessions))
  counts.update((r['state'],r['wait_event_type'],r['wait_event']) for r in sessions)
 print(role,'mean non-idle sessions',round(statistics.mean(active),2) if active else 0,'mean running',round(statistics.mean(running),2) if running else 0,'wait counts',counts.most_common(12))
if (p/'server-after.json').exists():
 before=json.loads((p/'server-before.json').read_text());after=json.loads((p/'server-after.json').read_text())
 for key in ('wal','checkpointer','database'):
  delta={k:round(v-before[key][k],2) for k,v in after[key].items() if isinstance(v,(int,float)) and isinstance(before[key].get(k),(int,float))}
  print(key,delta)
 for row in after['io']:
  old=next(r for r in before['io'] if all(r[k]==row[k] for k in ('backend_type','object','context')))
  delta={k:round(v-old[k],2) for k,v in row.items() if isinstance(v,(int,float)) and isinstance(old.get(k),(int,float)) and v!=old[k]}
  if delta:print('IO',row['backend_type'],row['object'],row['context'],delta)
if (p/'actual-batches.txt').exists():print('batches', (p/'actual-batches.txt').read_text())

monitor=[json.loads(x) for x in (p/'monitor.jsonl').read_text().splitlines()]
if len(monitor)>3:
 totals=collections.Counter();previous={}
 for row in monitor:
  current={}
  for line in row['processes'].splitlines():
   parts=line.split(None,5)
   if len(parts)!=6:continue
   pid=parts[0];t=parts[4].split(':')
   cpu=sum(float(v)*60**i for i,v in enumerate(reversed(t)))
   group='apps' if 'sqlstreams-native-scratch' in parts[5] else 'postgres'
   current[pid]=cpu
   if pid in previous:totals[group]+=max(0,cpu-previous[pid])
  previous=current
 elapsed=monitor[-1]['elapsed']-monitor[0]['elapsed']
 print('CPU mean percent of one core',{k:round(v/elapsed*100,1) for k,v in totals.items()})

if (p/'summary.json').exists():
 summary=json.loads((p/'summary.json').read_text())
 ends=[timestamp(r['at']) for r in summary['producers']]
 starts=[timestamp(r['at'])-r['elapsed_s'] for r in summary['producers']]
 print('aggregate production rate',round(sum(r['completed'] for r in summary['producers'])/(max(ends)-min(starts))))
 print('consumer p99 conservative upper bound',-1 if any(r['p99_ms_upper_bound']<0 for r in summary['consumers']) else max((r['p99_ms_upper_bound'] for r in summary['consumers']),default=None))
if frames and 'cursor' in frames[0]:
 for label,field in [('visible minus saved settled head','settled_head'),('visible minus claimed','claimed')]:
  values=sorted(f['visible_head']-f['cursor'][field] for f in frames if f.get('visible_head') is not None and f.get('cursor'))
  if values:print(label,'ID distance median/p99/max',values[len(values)//2],values[int(len(values)*.99)],max(values))
 values=sorted(f['cursor']['settled_head']-f['cursor']['claimed'] for f in frames if f.get('cursor'))
 if values:print('saved settled minus claimed ID distance median/p99/max',values[len(values)//2],values[int(len(values)*.99)],max(values))
 print('consumer blocking samples',collections.Counter((s['wait_event'],s['query'].split('\n')[1].strip() if '\n' in s['query'] else s['query'][:80]) for f in frames for s in f['sessions'] or [] if s['application_name']=='scratch-consumer' and s.get('blocking_pids')))
