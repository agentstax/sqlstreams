import subprocess, pathlib, time, json, sys, os, signal, shutil, hashlib, ctypes, plistlib
root=os.environ.get('SQLSTREAMS_NATIVE_ROOT') or pathlib.Path('/private/tmp/vulkan-native-session-path.txt').read_text().strip()
root=pathlib.Path(root)
callers=int(sys.argv[1]);duration=int(sys.argv[2]);maximum=int(sys.argv[3]);transactions=int(sys.argv[4]) if len(sys.argv)>4 else 1
name='scratch_'+time.strftime('%H%M%S',time.gmtime())
out=root/name;out.mkdir()
other_roots=[pathlib.Path('/private/tmp/vulkan-page-builds'),pathlib.Path('/private/tmp/vulkan-native.ULMBSf'),pathlib.Path(__file__).resolve().parent/'results']
other_bytes=sum(int(subprocess.check_output(['du','-sk',str(path)],text=True).split()[0])*1024 for path in other_roots if path.exists() and path.resolve()!=root.resolve())
pg=os.environ.get('SQLSTREAMS_NATIVE_PG_BIN','/opt/homebrew/opt/postgresql@18/bin/')
base=['-h','127.0.0.1','-p','55439','-U','scratch']
def sql(query):
 return subprocess.check_output([pg+'psql',*base,'-d',name,'-At','-v','ON_ERROR_STOP=1','-c',query],text=True)
def latest(path):
 try:
  rows=[json.loads(x) for x in path.read_text().splitlines()]
  return next((x for x in reversed(rows) if x.get('kind') in ('sample','final')), {})
 except (FileNotFoundError,json.JSONDecodeError):return {}
subprocess.run([pg+'createdb',*base,name],check=True)
env=os.environ.copy();env.update(GOGC=os.environ.get('GOGC','400'),GOMEMLIMIT=os.environ.get('GOMEMLIMIT','2GiB'))
exe=os.environ.get('SCRATCH_BINARY','/private/tmp/vulkan-native-scratch')
if os.environ.get('BENCH_PHASE'):
 (out/'phase.json').write_text(json.dumps(dict(study=os.environ.get('BENCH_STUDY'),phase=os.environ['BENCH_PHASE'])))
(out/'runner.sha256').write_text(hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest())
(out/'runner.py').write_bytes(pathlib.Path(__file__).read_bytes())
(out/'binary.sha256').write_text(hashlib.sha256(pathlib.Path(exe).read_bytes()).hexdigest())
consumer_env=dict(env,PGAPPNAME='scratch-consumer')
producer_env=dict(env,PGAPPNAME='scratch-producer')
consumer_args=['-claim',os.environ.get('CLAIM_SIZE','4000'),'-queue',os.environ.get('QUEUE_SIZE','16000'),'-handlers',os.environ.get('HANDLERS','4'),'-poll',os.environ.get('CLAIM_POLL','20ms'),'-connections',os.environ.get('CONSUMER_POOL','32'),'-minimum-connections',os.environ.get('CONSUMER_MIN_POOL','0')]
consumer_args+=['-plan-cache-mode',os.environ.get('CONSUMER_PLAN_MODE','auto')]
profile_args=['-contention'] if os.environ.get('CONTENTION')=='1' else []
server_sql="SELECT json_build_object('at',clock_timestamp(),'wal',(SELECT row_to_json(w) FROM pg_stat_wal w),'io',(SELECT json_agg(i) FROM pg_stat_io i),'checkpointer',(SELECT row_to_json(c) FROM pg_stat_checkpointer c),'database',(SELECT row_to_json(d) FROM pg_stat_database d WHERE datname=current_database()))"
defer_consumer=os.environ.get('DEFER_CONSUMER')=='1'
analyze_interval=float(os.environ.get('ANALYZE_INTERVAL','0'))
maintenance_mode=os.environ.get('MAINTENANCE_TIMING','')
assert maintenance_mode in ('','during','after')
maintenance_compression=os.environ.get('MAINTENANCE_COMPRESSION','off')
assert maintenance_compression in ('off','lz4')
maintenance_delay=float(os.environ.get('MAINTENANCE_DELAY_MS','2'))
assert maintenance_delay in (0.2,2)
maintenance=None;maintenance_file=None;maintenance_started=None
watch=None
iostat=None;disk_probe=None;diagnostic_files=[]
rca=os.environ.get('RCA')=='1'
backend_io=os.environ.get('BACKEND_IO')=='1'
if backend_io:
 assert rca
 process_library=ctypes.CDLL('/usr/lib/libproc.dylib',use_errno=True)
 process_library.proc_pid_rusage.argtypes=[ctypes.c_int,ctypes.c_int,ctypes.c_void_p]
 process_library.proc_pid_rusage.restype=ctypes.c_int
extra_producers=[];extra_consumers=[];extra_files=[]
producer_count=int(os.environ.get('PRODUCERS','1'));consumer_count=int(os.environ.get('CONSUMERS','1'))
assert 1<=producer_count<=4 and 0<=consumer_count<=4
assert maximum%producer_count==0
identity_limit=maximum//producer_count
retention_enabled=os.environ.get('RETENTION_TTL','0s')!='0s'
assert not retention_enabled or producer_count==consumer_count==1
host_args=['-host',os.environ.get('PGHOST','127.0.0.1')]
consumer_env['GOMAXPROCS']=os.environ.get('CONSUMER_PROCS',env.get('GOMAXPROCS','10'))
producer_env['GOMAXPROCS']=os.environ.get('PRODUCER_PROCS',env.get('GOMAXPROCS','10'))
for role,role_env in [('PRODUCER',producer_env),('CONSUMER',consumer_env)]:
 for setting in ['GOGC','GOMEMLIMIT']:
  role_env[setting]=os.environ.get(role+'_'+setting,env[setting])

retention_args=['-retention-ttl',os.environ.get('RETENTION_TTL','0s'),'-idempotency-ttl',os.environ.get('IDEMPOTENCY_TTL','24h')] if 'RETENTION_TTL' in os.environ or 'IDEMPOTENCY_TTL' in os.environ else []
with (out/'setup.log').open('w') as f:subprocess.run([exe,'-role','setup','-database',name,'-partition-size',os.environ.get('PARTITION_SIZE','5000000')]+retention_args,stdout=f,stderr=f,env=env,check=True)
(out/'stream-settings.json').write_text(sql('SELECT row_to_json(t) FROM sqlstreams.stream_config t WHERE name=\'orders\''))
if rca:
 source=pathlib.Path('pkg/produce/controller/datastore/insert.go')
 manifest=json.loads(pathlib.Path('/private/tmp/vulkan-native-sql-source.json').read_text())
 assert hashlib.sha256(source.read_bytes()).hexdigest()==manifest['sha256'], 'production SQL changed; rebuild the frozen control'
 shutil.copy('/private/tmp/vulkan-native-protected-insert.sql',out/'protected-insert.sql')
 (out/'sql-source.json').write_text(json.dumps(manifest))
sql("UPDATE sqlstreams.worker_config SET target_instances=0,updated_at=now() WHERE name='exception_consumer'; CREATE EXTENSION pg_stat_statements;")
if os.environ.get('JANITOR_POLL_SECONDS'):
 janitor_poll=int(os.environ['JANITOR_POLL_SECONDS'])
 assert 1<=janitor_poll<=300
 sql("UPDATE sqlstreams.worker_config SET metadata=jsonb_set(metadata,'{poll_rate}',to_jsonb("+str(janitor_poll*1000000000)+"::bigint)),updated_at=now() WHERE name='stream_janitor' AND stream_id=4")
if os.environ.get('BUFFER_SUMMARY')=='1':sql('CREATE EXTENSION pg_buffercache')
if maintenance_mode:
 assert maximum<20000000 and not consumer_count
 for table in ['message_log_4_0','idempotency_key_4']:
  sql('ALTER TABLE sqlstreams.'+table+' SET (autovacuum_vacuum_insert_threshold=20000000,autovacuum_vacuum_insert_scale_factor=0,autovacuum_analyze_threshold=20000000,autovacuum_analyze_scale_factor=0)')
 (out/'maintenance-config.json').write_text(json.dumps(dict(mode=maintenance_mode,start_after_s=15 if maintenance_mode=='during' else duration,cost_delay_ms=maintenance_delay,cost_limit=200,wal_compression=maintenance_compression,thresholds=json.loads(sql("SELECT json_agg(s) FROM (SELECT relname,reloptions FROM pg_class WHERE relnamespace='sqlstreams'::regnamespace AND reloptions IS NOT NULL) s")))))
 maintenance_file=(out/'maintenance.log').open('w')
 maintenance_command=[pg+'psql',*base,'-d',name,'-v','ON_ERROR_STOP=1','-c',"SET vacuum_cost_delay='"+str(maintenance_delay)+"ms'",'-c','SET vacuum_cost_limit=200','-c',"SET wal_compression='"+maintenance_compression+"'",'-c','SHOW wal_compression','-c','VACUUM (ANALYZE, VERBOSE) sqlstreams.message_log_4_0']

(out/'settings.txt').write_text(sql("SELECT name,setting FROM pg_settings WHERE name IN ('fsync','synchronous_commit','full_page_writes','autovacuum','jit','shared_buffers','max_wal_size','max_connections','port','data_checksums','wal_sync_method','io_method','track_wal_io_timing','work_mem','checkpoint_timeout'); SELECT version();"))
(out/'worker-settings.json').write_text(sql('SELECT json_agg(w) FROM sqlstreams.worker_config w'))
(out/'postgres-settings.json').write_text(sql("SELECT json_agg(s) FROM (SELECT name,setting,unit,source FROM pg_settings ORDER BY name) s"))
(out/'source.txt').write_text(subprocess.check_output(['git','rev-parse','HEAD'],text=True)+subprocess.check_output(['git','status','--short'],text=True))
consumer_log=(out/'consumer.jsonl').open('w');consumer_err=(out/'consumer.log').open('w')
producer_log=(out/'producer.jsonl').open('w');producer_err=(out/'producer.log').open('w')
consumer=None
if consumer_count:
 consumer=subprocess.Popen([exe,'-role','consumer','-database',name,'-maximum',str(maximum),'-profile',str(out/'consumer.cpu')]+consumer_args+profile_args+host_args+['-seen-file',str(out/'consumer.seen')],stdout=consumer_log,stderr=consumer_err,env=consumer_env)
producer=None
try:
 if consumer_count:
  for i in range(20):
   time.sleep(.5)
   if consumer.poll() is not None:raise RuntimeError('consumer stopped; inspect '+str(out))
   active=sql("SELECT count(*) FROM sqlstreams.worker_instance i JOIN sqlstreams.worker_config w ON w.id=i.worker_id WHERE w.name='message_consumer'").strip()
   if int(active)>0:break
  else:raise RuntimeError('consumer did not start')
  consumer.send_signal(signal.SIGTERM);consumer.wait(timeout=60)
  sql("UPDATE sqlstreams.worker_config SET target_instances=0,updated_at=now() WHERE name='exception_consumer'")
  consumer_log.seek(0);consumer_log.truncate();consumer_err.seek(0);consumer_err.truncate()
  if defer_consumer:consumer_args+=['-start-file',str(out/'start-consumer')]
  consumer=subprocess.Popen([exe,'-role','consumer','-database',name,'-maximum',str(maximum),'-profile',str(out/'consumer.cpu')]+consumer_args+profile_args+host_args+['-seen-file',str(out/'consumer.seen')],stdout=consumer_log,stderr=consumer_err,env=consumer_env)
  for i in range(1,consumer_count):
   log=(out/f'consumer-{i}.jsonl').open('w');err=(out/f'consumer-{i}.log').open('w');extra_files.extend([log,err])
   extra_consumers.append(subprocess.Popen([exe,'-role','consumer','-database',name,'-maximum',str(maximum),'-profile',str(out/f'consumer-{i}.cpu'),'-seen-file',str(out/f'consumer-{i}.seen')]+consumer_args+profile_args+host_args,stdout=log,stderr=err,env=consumer_env))
  time.sleep(2)
 exceptions=sql("SELECT count(*) FROM sqlstreams.worker_instance i JOIN sqlstreams.worker_config w ON w.id=i.worker_id WHERE w.name='exception_consumer'").strip()
 assert int(exceptions)==0,exceptions
 if not consumer_count:
  assert int(sql('SELECT count(*) FROM sqlstreams.worker_instance').strip())==0, 'worker process active'
 sql('CHECKPOINT')
 sql('SELECT pg_stat_statements_reset()')
 (out/'server-before.json').write_text(sql(server_sql))
 if rca:
  disk_file=(out/'iostat.txt').open('w');diagnostic_files.append(disk_file)
  (out/'iostat-start.json').write_text(json.dumps(dict(at=time.time(),first_row='since boot; exclude from interval analysis')))
  iostat=subprocess.Popen(['/usr/sbin/iostat','-w','1'],stdout=disk_file,stderr=subprocess.STDOUT)
  if os.environ.get('DISK_PROBE')=='1':
   probe_file=(out/'disk-probe.jsonl').open('w');diagnostic_files.append(probe_file)
   disk_probe=subprocess.Popen([sys.executable,'.bench/scratchnative/durable_probe.py','--directory',str(root),'--seconds',str(duration+15),'--rate','10'],stdout=probe_file,stderr=subprocess.STDOUT)
   for attempt in range(100):
    if (out/'disk-probe.jsonl').stat().st_size:break
    if disk_probe.poll() is not None:raise RuntimeError('disk probe initialization failed')
    time.sleep(.1)
   else:raise RuntimeError('disk probe did not initialize')
 watch_sql="SELECT json_build_object('at',clock_timestamp(),'cursor',(SELECT row_to_json(c) FROM (SELECT claimed,committed,settled_head,pending_head,pending_xmax::text FROM sqlstreams.consumer_group_cursor_4 LIMIT 1) c),'visible_head',(SELECT max(id) FROM sqlstreams.message_log_4),'snapshot_xmin',pg_snapshot_xmin(pg_current_snapshot())::text,'sessions',(SELECT json_agg(s) FROM (SELECT pid,datname,backend_type,application_name,state,wait_event_type,wait_event,EXTRACT(EPOCH FROM clock_timestamp()-xact_start) transaction_age_s,client_addr::text,backend_xid::text,backend_xmin::text,CASE WHEN wait_event_type='Lock' THEN pg_blocking_pids(pid) ELSE ARRAY[]::int[] END blocking_pids,left(query,300) query FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datname=current_database() OR backend_xid IS NOT NULL)) s))::jsonb;"
 (out/'watch.sql').write_text('\\pset tuples_only on\n'+watch_sql+'\n\\watch 0.1\n')
 watch_file=(out/'waits.jsonl').open('w')
 watch=subprocess.Popen([pg+'psql',*base,'-d',name,'-At','-f',str(out/'watch.sql')],stdout=watch_file,stderr=subprocess.STDOUT,env=dict(env,PGAPPNAME='scratch-monitor'))
 command=[exe,'-role','producer','-database',name,'-callers',str(callers),'-duration',str(duration)+'s','-maximum',str(identity_limit),'-transactions',str(transactions),'-profile',str(out/'producer.cpu'),'-batch',os.environ.get('BATCH_SIZE','1000')]+profile_args+host_args
 if os.environ.get('CPU_PROFILE','1')=='0':
  position=command.index('-profile');del command[position:position+2]
 if os.environ.get('EXPLICIT_BATCH')=='1':command.append('-explicit-batch')
 command+=['-connections',os.environ.get('POOL_CONNECTIONS','32'),'-minimum-connections',os.environ.get('PRODUCER_MIN_POOL','0'),'-rate',os.environ.get('PRODUCER_RATE','0')]
 if os.environ.get('PRODUCER_RATE_SCHEDULE'):command+=['-rate-schedule',os.environ['PRODUCER_RATE_SCHEDULE']]
 command+=['-query-mode',os.environ.get('QUERY_MODE','cache_statement'),'-statement-cache',os.environ.get('STATEMENT_CACHE','512')]
 if os.environ.get('RAW_PGX')=='1':
  assert rca
  command+=['-raw-sql',str(out/'protected-insert.sql'),'-stream-id',str(json.loads((out/'stream-settings.json').read_text())['id'])]
 (out/'command.json').write_text(json.dumps(dict(command=command,environment={k:env.get(k) for k in ('GOGC','GOMEMLIMIT','GOMAXPROCS')},consumer_args=consumer_args,producer_count=producer_count,consumer_count=consumer_count,producer_runtime={k:producer_env.get(k) for k in ('GOMAXPROCS','GOGC','GOMEMLIMIT')},consumer_runtime={k:consumer_env.get(k) for k in ('GOMAXPROCS','GOGC','GOMEMLIMIT')},producer_gomaxprocs=producer_env['GOMAXPROCS'],consumer_gomaxprocs=consumer_env['GOMAXPROCS'],defer_consumer=defer_consumer,checkpoint_before_run=True,analyze_interval=analyze_interval)))
 producer=subprocess.Popen(command,stdout=producer_log,stderr=producer_err,env=producer_env)
 for i in range(1,producer_count):
  log=(out/f'producer-{i}.jsonl').open('w');err=(out/f'producer-{i}.log').open('w');extra_files.extend([log,err])
  extra_command=command.copy();extra_command[extra_command.index('-maximum')+1]=str(identity_limit*(i+1))
  if '-profile' in extra_command:extra_command[extra_command.index('-profile')+1]=str(out/f'producer-{i}.cpu')
  extra_producers.append(subprocess.Popen(extra_command+['-sequence-start',str(identity_limit*i)],stdout=log,stderr=err,env=producer_env))
 (out/'pids.json').write_text(json.dumps(dict(producer=producer.pid,consumer=consumer.pid if consumer else None,extra_producers=[p.pid for p in extra_producers],extra_consumers=[p.pid for p in extra_consumers])))
 print(str(out),flush=True)
 with (out/'monitor.jsonl').open('w') as monitor:
  started=time.monotonic();next_analyze=analyze_interval;next_buffers=0
  while any(p.poll() is None for p in [producer]+extra_producers):
   if maintenance_mode=='during' and maintenance is None and time.monotonic()-started>=15:
    maintenance_started=time.time()
    maintenance=subprocess.Popen(maintenance_command,stdout=maintenance_file,stderr=subprocess.STDOUT,env=dict(env,PGAPPNAME='scratch-maintenance'))
    (out/'maintenance-start.json').write_text(json.dumps(dict(at=maintenance_started,pid=maintenance.pid)))

   if analyze_interval and time.monotonic()-started>=next_analyze:
    analyze_started=time.monotonic()
    analyze_query='ANALYZE sqlstreams.idempotency_key_4 (created_at)' if os.environ.get('ANALYZE_IDEMPOTENCY_ONLY')=='1' else 'ANALYZE sqlstreams.message_log_4; ANALYZE sqlstreams.idempotency_key_4'
    sql(analyze_query)
    with (out/'maintenance.jsonl').open('a') as analysis_file:analysis_file.write(json.dumps(dict(at=time.time(),elapsed=time.monotonic()-started,duration_s=time.monotonic()-analyze_started,query=analyze_query,once=os.environ.get('ANALYZE_ONCE')=='1'))+'\n')
    next_analyze=float('inf') if os.environ.get('ANALYZE_ONCE')=='1' else time.monotonic()-started+analyze_interval
   disk_kib=int(subprocess.check_output(['du','-sk',str(root)],text=True).split()[0])
   free=shutil.disk_usage(root).free
   if disk_kib*1024+other_bytes>100*1000**3 or disk_kib>int(os.environ.get('STORAGE_LIMIT_KIB','85000000')) or free<40*1024**3:raise RuntimeError('storage guard reached')
   snapshot=sql("SELECT (SELECT max(id) FROM sqlstreams.message_log_4),(SELECT max(committed) FROM sqlstreams.consumer_group_cursor_4); SELECT wait_event_type,wait_event,count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active' GROUP BY 1,2")
   table_stats=sql("SELECT json_agg(s) FROM (SELECT relname,n_live_tup,n_dead_tup,n_tup_ins,n_tup_del,n_mod_since_analyze,n_ins_since_vacuum,last_vacuum,last_autovacuum,vacuum_count,autovacuum_count,last_analyze,last_autoanalyze,analyze_count,autoanalyze_count FROM pg_stat_user_tables WHERE schemaname='sqlstreams' AND (relname='idempotency_key_4' OR relname LIKE 'message_log_4%')) s")
   cpu=subprocess.check_output(['ps','-axo','pid,ppid,%cpu,rss,time,comm'],text=True)
   relevant='\n'.join(line for line in cpu.splitlines() if 'postgres' in line or 'sqlstreams-native-scratch' in line)
   row=dict(at=time.time(),elapsed=time.monotonic()-started,database=snapshot,table_stats=json.loads(table_stats),processes=relevant,host_processes=cpu,storage_kib=disk_kib,free_bytes=free)
   if retention_enabled:
    row['retention']=json.loads(sql("SELECT json_build_object('database_bytes',pg_database_size(current_database()),'key_bytes',pg_total_relation_size('sqlstreams.idempotency_key_4'),'partitions',(SELECT json_agg(c.relname ORDER BY c.relname) FROM pg_inherits i JOIN pg_class c ON c.oid=i.inhrelid WHERE i.inhparent='sqlstreams.message_log_4'::regclass))"))
   if rca or os.environ.get('STEADY_STATS')=='1':row['server_stats']=json.loads(sql(server_sql))
   if os.environ.get('DEVICE_IO')=='1':
    device=plistlib.loads(subprocess.check_output(['/usr/sbin/ioreg','-a','-r','-c','IOBlockStorageDriver']))
    volumes=plistlib.loads(subprocess.check_output(['/usr/sbin/ioreg','-a','-r','-c','AppleAPFSVolume']))
    row['device_io']=[dict(name=item.get('BSD Name',item['IORegistryEntryName']),statistics=item['Statistics']) for item in device+volumes if 'Statistics' in item and (item.get('IOObjectClass')=='IOBlockStorageDriver' or item.get('Role')==['Data'])]
    row['device_io_completed_at']=time.time()
   if maintenance_mode:
    row['vacuum_progress']=json.loads(sql("SELECT coalesce(json_agg(s),'[]'::json) FROM pg_stat_progress_vacuum s"))
   if backend_io:
    row['backend_io']=json.loads(sql("SELECT json_agg(json_build_object('pid',a.pid,'backend_type',a.backend_type,'application_name',a.application_name,'backend_start',a.backend_start,'io',(SELECT json_agg(i) FROM pg_stat_get_backend_io(a.pid) i))) FROM pg_stat_activity a WHERE a.pid<>pg_backend_pid()"))
    row['relation_io']=json.loads(sql("SELECT json_agg(s) FROM pg_statio_user_tables s WHERE schemaname='sqlstreams'"))
    row['index_io']=json.loads(sql("SELECT json_agg(s) FROM pg_statio_user_indexes s WHERE schemaname='sqlstreams'"))
    records=[];unavailable=0
    for line in cpu.splitlines()[1:]:
     fields=line.split(None,5)
     if len(fields)!=6:continue
     buffer=ctypes.create_string_buffer(160)
     if process_library.proc_pid_rusage(int(fields[0]),2,buffer):
      unavailable+=1;continue
     values=(ctypes.c_uint64*18).from_buffer_copy(buffer.raw[16:])
     records.append(dict(pid=int(fields[0]),command=fields[5],start=values[8],user_ns=values[0],system_ns=values[1],read_bytes=values[16],write_bytes=values[17]))
    row['host_io']=records;row['host_io_unavailable']=unavailable
    row['backend_io_completed_at']=time.time()

   if os.environ.get('BUFFER_SUMMARY')=='1' and time.monotonic()-started>=next_buffers:
    row['buffer_summary']=json.loads(sql('SELECT row_to_json(s) FROM public.pg_buffercache_summary() s'))
    next_buffers=time.monotonic()-started+5
   if os.environ.get('VM_STATS')=='1' or 'buffer_summary' in row:
    row['vm_stat']=subprocess.check_output(['/usr/bin/vm_stat'],text=True)
   monitor.write(json.dumps(row)+'\n');monitor.flush()
   time.sleep(1)
  if any(p.returncode for p in [producer]+extra_producers):raise RuntimeError('producer stopped with an error')
  (out/'server-after.json').write_text(sql(server_sql))
  for process in (disk_probe,iostat):
   if process and process.poll() is None:process.send_signal(signal.SIGTERM);process.wait(timeout=15)
  watch.send_signal(signal.SIGTERM);watch.wait(timeout=10);watch_file.close()
  if defer_consumer:(out/'start-consumer').touch()
  target=sum(latest(path)['completed'] for path in out.glob('producer*.jsonl'))
  if consumer_count:
   deadline=time.monotonic()+60
   while sum(latest(path).get('completed',0) for path in out.glob('consumer*.jsonl'))<target and time.monotonic()<deadline:
    if consumer.poll() is not None:raise RuntimeError('consumer stopped early')
    time.sleep(.5)
   time.sleep(1)
   for process in [consumer]+extra_consumers:
    process.send_signal(signal.SIGTERM);process.wait(timeout=60)
 if maintenance_mode:
  if maintenance is None:
   maintenance_started=time.time()
   maintenance=subprocess.Popen(maintenance_command,stdout=maintenance_file,stderr=subprocess.STDOUT,env=dict(env,PGAPPNAME='scratch-maintenance'))
   (out/'maintenance-start.json').write_text(json.dumps(dict(at=maintenance_started,pid=maintenance.pid)))
  maintenance.wait(timeout=900)
  assert maintenance.returncode==0,'controlled maintenance failed'
  maintenance_file.flush()
  (out/'maintenance-after.json').write_text(sql("SELECT json_agg(s) FROM (SELECT relname,n_ins_since_vacuum,last_vacuum,last_autovacuum,vacuum_count,autovacuum_count,last_analyze,analyze_count FROM pg_stat_user_tables WHERE relname IN ('message_log_4_0','idempotency_key_4')) s"))
 (out/'sql-profile.txt').write_text(sql("SELECT calls,round(total_exec_time::numeric) total_ms,rows,left(query,160) FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 15"))
 (out/'sql-profile.json').write_text(sql("SELECT json_agg(s) FROM (SELECT calls,total_exec_time,rows,temp_blks_read,temp_blks_written,shared_blks_read,shared_blks_hit,wal_bytes,query FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 20) s"))
 (out/'cleanup-profile.json').write_text(sql("SELECT json_agg(s) FROM (SELECT calls,total_exec_time,rows,shared_blks_read,shared_blks_hit,query FROM pg_stat_statements WHERE query LIKE 'DELETE FROM sqlstreams.idempotency_key_4%') s"))
 (out/'actual-batches.txt').write_text(sql("SELECT count(*) transactions,min(messages),round(avg(messages),2),max(messages) FROM (SELECT xmin,count(*) messages FROM sqlstreams.message_log_4 GROUP BY xmin) batches"))
 (out/'verification.txt').write_text(sql("SELECT count(*) FROM sqlstreams.message_log_4; SELECT count(*) FROM sqlstreams.worker_instance i JOIN sqlstreams.worker_config w ON w.id=i.worker_id WHERE w.name='exception_consumer'; SELECT name,target_instances FROM sqlstreams.worker_config WHERE name='exception_consumer'"))
 if rca and os.environ.get('VERIFY_SEMANTICS')=='1':
  (out/'semantics.json').write_text(sql("SELECT json_build_object('messages',(SELECT count(*) FROM sqlstreams.message_log_4),'keys',(SELECT count(*) FROM sqlstreams.idempotency_key_4),'nondefault_messages',(SELECT count(*) FROM sqlstreams.message_log_4 WHERE options IS NOT NULL OR routing_key IS NOT NULL OR message_key IS NOT NULL OR schema_version<>1),'non_v7_keys',(SELECT count(*) FROM sqlstreams.idempotency_key_4 WHERE substring(idempotency_key::text,15,1)<>'7'))"))
  semantics=json.loads((out/'semantics.json').read_text())
  assert semantics['messages']==semantics['keys'] and semantics['nondefault_messages']==semantics['non_v7_keys']==0
finally:
 if maintenance and maintenance.poll() is None:maintenance.terminate();maintenance.wait(timeout=15)
 if maintenance_file:maintenance_file.close()
 for process in (disk_probe,iostat):
  if process and process.poll() is None:process.send_signal(signal.SIGTERM);process.wait(timeout=15)
 for file in diagnostic_files:file.close()
 for process in extra_producers+extra_consumers:
  if process.poll() is None:process.send_signal(signal.SIGTERM);process.wait(timeout=60)
 for file in extra_files:file.close()
 if watch and watch.poll() is None:watch.send_signal(signal.SIGTERM);watch.wait(timeout=10);watch_file.close()
 if producer and producer.poll() is None:producer.send_signal(signal.SIGTERM);producer.wait(timeout=60)
 if consumer and consumer.poll() is None:consumer.send_signal(signal.SIGTERM);consumer.wait(timeout=60)
 for f in [consumer_log,consumer_err,producer_log,producer_err]:f.close()
produced=[latest(path) for path in sorted(out.glob('producer*.jsonl'))];consumed=[latest(path) for path in sorted(out.glob('consumer*.jsonl')) if path.stat().st_size]
summary=dict(producers=produced,consumers=consumed)
(out/'summary.json').write_text(json.dumps(summary))
print(json.dumps(summary),flush=True)
if not retention_enabled:
 assert sum(p['completed'] for p in produced)==int((out/'verification.txt').read_text().splitlines()[0]), 'database count mismatch'
else:
 retained=int((out/'verification.txt').read_text().splitlines()[0])
 assert 0<=retained<=sum(p['completed'] for p in produced), 'retained database count exceeds production'
if consumer_count:assert sum(p['completed'] for p in produced)==sum(c['completed'] for c in consumed), 'consumer count mismatch'
assert all(r['errors']==r['duplicates']==0 for r in produced+consumed), 'errors or duplicates'
seen=0
for path in sorted(out.glob('consumer*.seen')):
 bits=int.from_bytes(path.read_bytes(),'little')
 assert seen & bits == 0, 'duplicate across consumer processes'
 seen |= bits
assert bin(seen).count('1')==sum(c['completed'] for c in consumed), 'identity count mismatch'
if retention_enabled:
 assert seen==((1<<(sum(p['completed'] for p in produced)+1))-2), 'missing or unexpected produced identity after retention'
 (out/'retention-verification.json').write_text(json.dumps(dict(produced=sum(p['completed'] for p in produced),retained_rows=retained,all_produced_identities_consumed_once=True,actual_batches_scope='surviving rows only',retention_ttl=os.environ['RETENTION_TTL'],idempotency_ttl=os.environ.get('IDEMPOTENCY_TTL','24h'))))
assert int((out/'verification.txt').read_text().splitlines()[1])==0, 'exception consumer active'
if not consumer_count and os.environ.get('VERIFY_IDENTITIES')=='1':
 identity_counts=json.loads(sql("SELECT json_build_object('rows',count(*),'distinct_sequences',count(DISTINCT (payload->>'sequence')::bigint),'minimum_sequence',min((payload->>'sequence')::bigint),'maximum_sequence',max((payload->>'sequence')::bigint)) FROM sqlstreams.message_log_4"))
 (out/'producer-identities.json').write_text(json.dumps(identity_counts))
 assert identity_counts['rows']==identity_counts['distinct_sequences']==sum(p['completed'] for p in produced), 'producer identity mismatch'
if not consumer_count:
 assert int(sql('SELECT count(*) FROM sqlstreams.worker_instance').strip())==0, 'worker became active'
 (out/'producer-only.txt').write_text('No consumer processes launched; worker_instance empty before and after production.\n')
if os.environ.get('PLAN_PROBE')=='1':
 (out/'cleanup-catalog.json').write_text(sql("SELECT json_build_object('indexes',(SELECT json_agg(i) FROM pg_indexes i WHERE schemaname='sqlstreams' AND tablename='idempotency_key_4'),'statistics',(SELECT json_agg(s) FROM pg_stats s WHERE schemaname='sqlstreams' AND tablename='idempotency_key_4'),'table',(SELECT row_to_json(t) FROM pg_stat_user_tables t WHERE schemaname='sqlstreams' AND relname='idempotency_key_4'))"))
 for mode in ['force_generic_plan','force_custom_plan']:
  query="BEGIN; SET LOCAL plan_cache_mode='"+mode+"'; PREPARE cleanup_probe(timestamptz,bigint) AS DELETE FROM sqlstreams.idempotency_key_4 WHERE idempotency_key IN (SELECT idempotency_key FROM sqlstreams.idempotency_key_4 WHERE created_at < $1 LIMIT $2); EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) EXECUTE cleanup_probe(now()-interval '24 hours',1000); ROLLBACK;"
  (out/('cleanup-plan-'+mode+'.txt')).write_text(sql(query))
subprocess.run([pg+'dropdb',*base,name],check=True)
print('Scratch database removed; native cluster and evidence retained.',flush=True)

results=pathlib.Path(__file__).resolve().parent/'results'
evidence=results/'evidence'/'native18'/name
evidence.parent.mkdir(parents=True,exist_ok=True)
shutil.move(str(out),str(evidence));out.symlink_to(evidence,target_is_directory=True)
record=dict(run=name,evidence=str(evidence.relative_to(results.parents[2])),command=json.loads((evidence/'command.json').read_text()),producers=produced,consumers=consumed,verification=(evidence/'verification.txt').read_text(),sustainable_maximum_established=False)
with (results/'runs.jsonl').open('a') as index:index.write(json.dumps(record)+'\n')
print('Evidence retained at '+str(evidence),flush=True)
