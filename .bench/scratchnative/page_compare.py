"""Run one page-size variant at a time and restore the native baseline."""
import argparse, hashlib, json, os, pathlib, subprocess, sys

parser = argparse.ArgumentParser()
parser.add_argument('--wal-recycling',action='store_true')
parser.add_argument('--direct-io',action='store_true')
parser.add_argument('--device-io',action='store_true')
arguments = parser.parse_args()
assert sum([arguments.wal_recycling,arguments.direct_io,arguments.device_io])<=1
root = pathlib.Path('/private/tmp/vulkan-page-builds')
baseline = pathlib.Path('/private/tmp/vulkan-native18.D1I5CE')
baseline_bin = '/opt/homebrew/opt/postgresql@18/bin/'
archive = pathlib.Path('.bench/scratchnative/results/evidence/native18/page_builds')
archive.mkdir(exist_ok=True)
for block in [8,16]:
    binary = root/('install'+str(block))/'bin'
    data = root/('data'+str(block))
    if not data.exists():
        with (root/('init'+str(block)+'.log')).open('w') as output:
            subprocess.run([str(binary/'initdb'),'-D',str(data),'-U','scratch','-A','trust','--locale=C','--encoding=UTF8','--data-checksums'],stdout=output,stderr=subprocess.STDOUT,check=True)
        with (data/'postgresql.conf').open('a') as output:
            output.write("\nlisten_addresses='127.0.0.1'\nport=55439\nshared_buffers='6GB'\nwal_buffers='16MB'\nmax_wal_size='8GB'\nmin_wal_size='2GB'\ncheckpoint_completion_target=0.9\nmax_connections=200\nshared_preload_libraries='pg_stat_statements'\ntrack_io_timing=on\ntrack_wal_io_timing=on\njit=off\nfsync=on\nsynchronous_commit=on\nfull_page_writes=on\nautovacuum=on\nwork_mem='32MB'\nmaintenance_work_mem='64MB'\nbgwriter_delay='200ms'\nio_combine_limit='128kB'\nio_max_combine_limit='128kB'\nvacuum_buffer_usage_limit='2MB'\n")
            output.write('bgwriter_lru_maxpages='+str(100*8//block)+'\n')
    (archive/('build'+str(block)+'.json')).write_bytes((root/('build'+str(block)+'.json')).read_bytes())
    (archive/('config'+str(block)+'.txt')).write_bytes((data/'postgresql.conf').read_bytes())
    (archive/('binary'+str(block)+'.sha256')).write_text(hashlib.sha256((binary/'postgres').read_bytes()).hexdigest())
    (archive/('pg_config'+str(block)+'.txt')).write_text(subprocess.check_output([str(binary/'pg_config'),'--configure'],text=True))
(archive/'source.sha256').write_bytes((root/'postgresql-18.6.tar.bz2.sha256').read_bytes())
subprocess.run([baseline_bin+'pg_ctl','-D',str(baseline/'pgdata'),'stop','-m','fast','-w'],check=True)
active = None
plan = [(block,seconds,maximum,'on','page_comparison') for block,seconds,maximum in [(8,3,10000),(16,3,10000),(8,45,12000000),(16,45,12000000),(16,45,12000000),(8,45,12000000)]]
if arguments.wal_recycling:
    plan = [(16,seconds,maximum,recycle,phase) for recycle in ['on','off','on'] for seconds,maximum,phase in [(90,8000000,'conditioning'),(45,12000000,'measurement')]]
    (archive/'wal_recycling_plan.json').write_text(json.dumps(plan))
direct_modes = ['']*len(plan)
if arguments.direct_io:
    direct_modes = ['', 'wal', 'data,wal', '']
    plan = [(16,45,12000000,'on','measurement') for _ in direct_modes]
    (archive/'direct_io_plan.json').write_text(json.dumps(dict(plan=plan,direct_modes=direct_modes,diagnostic_only=True)))
if arguments.device_io:
    plan=[(16,90,18000000,'on','measurement')]
    direct_modes=['']
try:
    for (block, seconds, maximum, recycle, phase), direct in zip(plan,direct_modes):
        binary = root/('install'+str(block))/'bin'; data = root/('data'+str(block))
        subprocess.run([str(binary/'pg_ctl'),'-D',str(data),'-l',str(root/('postgres'+str(block)+'.log')),'start','-w','-o','-c wal_recycle='+recycle+(' -c debug_io_direct='+direct if direct else '')],check=True)
        active = (binary,data)
        settings = subprocess.check_output([str(binary/'psql'),'-h','127.0.0.1','-p','55439','-U','scratch','-d','postgres','-At','-c',"SELECT current_setting('block_size'),pg_size_bytes(current_setting('shared_buffers')),pg_size_bytes(current_setting('wal_buffers')),current_setting('fsync'),current_setting('full_page_writes'),current_setting('synchronous_commit'),current_setting('data_checksums'),current_setting('wal_recycle'),current_setting('wal_init_zero')"],text=True).strip()
        assert settings==str(block*1024)+'|6442450944|16777216|on|on|on|on|'+recycle+'|on',settings
        actual_direct=subprocess.check_output([str(binary/'psql'),'-h','127.0.0.1','-p','55439','-U','scratch','-d','postgres','-At','-c','SHOW debug_io_direct'],text=True).strip()
        assert actual_direct==direct,actual_direct
        print('Verified settings '+settings,flush=True)
        environment = dict(os.environ,VULKAN_NATIVE_ROOT=str(root),VULKAN_NATIVE_PG_BIN=str(binary)+'/',STORAGE_LIMIT_KIB='75000000',RCA='1',BACKEND_IO='1',BUFFER_SUMMARY='1',CPU_PROFILE='0',CONSUMERS='0',EXPLICIT_BATCH='1',PARTITION_SIZE='20000000',MAINTENANCE_TIMING='',BENCH_PHASE=phase,BENCH_STUDY='wal_recycling' if arguments.wal_recycling else 'page_size')
        if arguments.direct_io:
            environment['BENCH_STUDY']='direct_io'
            environment['DISK_PROBE']='1'
        if arguments.device_io:
            environment.update(BENCH_STUDY='device_io',DISK_PROBE='1',DEVICE_IO='1')
        subprocess.run([sys.executable,'.bench/scratchnative/run.py','8',str(seconds),str(maximum),'8'],env=environment,check=True)
        subprocess.run([str(binary/'pg_ctl'),'-D',str(data),'stop','-m','fast','-w'],check=True)
        active = None
finally:
    if active:
        binary,data=active
        subprocess.run([str(binary/'pg_ctl'),'-D',str(data),'stop','-m','fast','-w'],check=True)
    subprocess.run([baseline_bin+'pg_ctl','-D',str(baseline/'pgdata'),'-l',str(baseline/'postgres.log'),'start','-w'],check=True)
