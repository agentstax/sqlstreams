"""Bounded storage-only control; no database files are opened or changed."""
import argparse, ctypes, fcntl, hashlib, json, mmap, os, pathlib, plistlib, shutil, subprocess, sys, time, struct, signal

parser = argparse.ArgumentParser()
parser.add_argument('--host-sample-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--extent-map', action='store_true')
parser.add_argument('--disk-policy-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--worker', choices=['wal', 'data'])
parser.add_argument('--directory', required=True)
parser.add_argument('--seconds', type=int, default=60)
parser.add_argument('--mixed', action='store_true')
parser.add_argument('--data-gib', type=int, choices=[8,16], default=8)
parser.add_argument('--data-block-kib', type=int, choices=[8,16,1024], default=8)
parser.add_argument('--data-offset-kib', type=int, choices=[0,8], default=0)
parser.add_argument('--wal-offset-kib', type=int, choices=[0,8], default=0)
parser.add_argument('--wal-gib', type=int, choices=[1,8,16], default=16)
parser.add_argument('--wal-prefill', action='store_true')
parser.add_argument('--wal-rate-mib', type=int, choices=[0,250,425,850,1200], default=250, help='MiB/s limit; zero runs without pacing')
parser.add_argument('--wal-nocache', action='store_true')
parser.add_argument('--wal-working-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--wal-cache-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--wal-offset-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--no-catchup', action='store_true')
parser.add_argument('--aligned-buffer', action='store_true')
parser.add_argument('--data-fsync-seconds', type=int, choices=[1,30], default=1)
parser.add_argument('--data-fsync-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--data-sync-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--data-sync', action='store_true')
parser.add_argument('--data-nocache', action='store_true')
parser.add_argument('--data-cache-switch-seconds', type=int, choices=[0,15], default=0)
parser.add_argument('--data-low-rate-mib', type=int, choices=[0,100], default=100)
parser.add_argument('--data-rate-switch-seconds', type=int, choices=[0,60], default=0)
parser.add_argument('--data-rate-mib', type=int, choices=[300,425,600], default=300)
parser.add_argument('--pause-after-seconds', type=int, choices=[0,120], default=0)
parser.add_argument('--pause-seconds', type=int, choices=[0,30], default=0)
arguments = parser.parse_args()
directory = pathlib.Path(arguments.directory)

def emit(value):
    print(json.dumps(value), flush=True)

if arguments.worker:
    role = arguments.worker
    policy_library=ctypes.CDLL(None,use_errno=True)
    policy_library.getiopolicy_np.argtypes=[ctypes.c_int,ctypes.c_int]
    policy_library.getiopolicy_np.restype=ctypes.c_int
    policy_library.setiopolicy_np.argtypes=[ctypes.c_int,ctypes.c_int,ctypes.c_int]
    policy_library.setiopolicy_np.restype=ctypes.c_int
    policy_library.qos_class_self.restype=ctypes.c_uint
    emit(dict(kind='io_policy',role=role,at=time.time(),pid=os.getpid(),process_policy=policy_library.getiopolicy_np(0,0),thread_policy=policy_library.getiopolicy_np(0,1),qos=policy_library.qos_class_self(),darwin_background=policy_library.getpriority(4,os.getpid())))
    task = ctypes.c_uint.in_dll(policy_library,'mach_task_self_').value
    policy_library.task_policy_get.argtypes=[ctypes.c_uint,ctypes.c_int,ctypes.c_void_p,ctypes.POINTER(ctypes.c_uint),ctypes.POINTER(ctypes.c_int)]
    for flavor, name in [(1,'category'),(3,'suppression'),(4,'effective_state')]:
        policy = (ctypes.c_uint*16)()
        count = ctypes.c_uint(16)
        default = ctypes.c_int(0)
        result = policy_library.task_policy_get(task,flavor,policy,ctypes.byref(count),ctypes.byref(default))
        emit(dict(kind='task_policy',at=time.time(),role=role,policy=name,returncode=result,count=count.value,raw=list(policy)))
    block = os.urandom(1024 * 1024 if role == 'wal' else arguments.data_block_kib * 1024)
    if arguments.aligned_buffer:
        aligned = mmap.mmap(-1, len(block))
        aligned[:] = block
        block = aligned
        assert ctypes.addressof(ctypes.c_char.from_buffer(block)) % mmap.PAGESIZE == 0
    capacity = (arguments.wal_gib if role == 'wal' else arguments.data_gib) * 1024**3
    rate = (arguments.wal_rate_mib if role == 'wal' else arguments.data_rate_mib) * 1024**2
    displacement = (arguments.wal_offset_kib if role == 'wal' else arguments.data_offset_kib) * 1024
    path = directory / (role + '.bin')
    descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_RDWR | (os.O_DSYNC if role == 'wal' or arguments.data_sync else 0), 0o600)
    if (role == 'data' and arguments.data_nocache) or (role == 'wal' and arguments.wal_nocache):
        # macOS SDK sys/fcntl.h: F_NOCACHE disables file data caching.
        fcntl.fcntl(descriptor,48,1)
    if role == 'wal' and arguments.wal_prefill:
        reserve = len(block) if arguments.wal_offset_switch_seconds else 0
        for offset in range(0,capacity+displacement+reserve,len(block)):
            assert os.pwrite(descriptor,block,offset)==len(block)
        os.fsync(descriptor)
        (directory/'wal-ready').touch()
    if role == 'data' and arguments.wal_prefill:
        while not (directory/'wal-ready').exists():
            time.sleep(0.05)
    started = previous = last_sync = time.monotonic()
    deadline = started
    completed = previous_bytes = 0
    latencies = []
    sync_ms = 0
    fsync_seconds = arguments.data_fsync_seconds
    nocache = (role=='data' and arguments.data_nocache) or (role=='wal' and arguments.wal_nocache)
    paused_seconds = 0
    working_capacity = capacity
    emit(dict(kind='config', role=role, pid=os.getpid(), at=time.time(), block_bytes=len(block), capacity_bytes=capacity, offset_bytes=displacement, prefilled=role=='wal' and arguments.wal_prefill, nocache=(role=='data' and arguments.data_nocache) or (role=='wal' and arguments.wal_nocache), aligned_buffer=arguments.aligned_buffer, memory_page_bytes=mmap.PAGESIZE, target_bytes_s=rate, catchup=not arguments.no_catchup, working_switch_seconds=arguments.wal_working_switch_seconds, sync='O_DSYNC' if role == 'wal' or arguments.data_sync else 'fsync every '+str(arguments.data_fsync_seconds)+' seconds'))
    disk_policy = policy_library.getiopolicy_np(0,0)
    data_synchronous = arguments.data_sync
    next_extent_sample = 0
    try:
        while time.monotonic() - started < arguments.seconds:
            if arguments.disk_policy_switch_seconds:
                requested = 2 if int((time.monotonic()-started)//arguments.disk_policy_switch_seconds)%2 else 0
                if requested != disk_policy:
                    assert policy_library.setiopolicy_np(0,0,requested)==0, ctypes.get_errno()
                    disk_policy = policy_library.getiopolicy_np(0,0)
                    assert disk_policy == requested
                    emit(dict(kind='disk_policy_switch',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,policy=disk_policy))
            if role == 'data' and arguments.data_sync_switch_seconds:
                requested = arguments.data_sync != bool(int((time.monotonic()-started)//arguments.data_sync_switch_seconds)%2)
                if requested != data_synchronous:
                    switch_started = time.monotonic()
                    os.fsync(descriptor)
                    replacement = os.open(path, os.O_RDWR | (os.O_DSYNC if requested else 0))
                    if nocache:
                        fcntl.fcntl(replacement,48,1)
                    flags = fcntl.fcntl(replacement,fcntl.F_GETFL)
                    assert bool(flags & os.O_DSYNC) == requested
                    os.close(descriptor)
                    descriptor = replacement
                    data_synchronous = requested
                    emit(dict(kind='data_sync_switch',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,synchronous=data_synchronous,flags=flags,switch_ms=(time.monotonic()-switch_started)*1000))
            if arguments.extent_map and time.monotonic()-started >= next_extent_sample:
                # Installed macOS fcntl.h: packed(4) log2phys, F_LOG2PHYS_EXT=65.
                extent_started = time.monotonic()
                size = os.fstat(descriptor).st_size
                offset = 0
                extents = []
                while offset < size:
                    flags, contiguous, physical = struct.unpack('=Iqq', fcntl.fcntl(descriptor,65,struct.pack('=Iqq',0,size-offset,offset)))
                    assert contiguous > 0 and physical >= 0
                    extents.append(dict(logical=offset, physical=physical, length=contiguous))
                    offset += contiguous
                    assert len(extents) < 65536
                emit(dict(kind='extents', at=time.time(), elapsed_s=time.monotonic()-started, duration_s=time.monotonic()-extent_started, size=size, extents=extents))
                next_extent_sample = time.monotonic()-started+30

            if role == 'data' and arguments.data_fsync_switch_seconds:
                requested = 30 if int((time.monotonic()-started)//arguments.data_fsync_switch_seconds)%2 else 1
                if requested != fsync_seconds:
                    fsync_seconds = requested
                    emit(dict(kind='fsync_cadence', at=time.time(), elapsed_s=time.monotonic()-started, seconds=fsync_seconds))
            if role == 'data' and arguments.data_rate_switch_seconds:
                requested = (arguments.data_low_rate_mib if int((time.monotonic()-started)//arguments.data_rate_switch_seconds)%2 else arguments.data_rate_mib) * 1024**2
                if requested != rate:
                    rate = requested
                    emit(dict(kind='data_rate_switch', at=time.time(), elapsed_s=time.monotonic()-started, bytes=completed, target_bytes_s=rate))
                    if rate == 0:
                        flush_started = time.monotonic()
                        os.fsync(descriptor)
                        emit(dict(kind='data_pause_ready',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,flush_ms=(time.monotonic()-flush_started)*1000))
                if rate == 0:
                    time.sleep(0.05)
                    continue
            if role=='wal' and arguments.wal_offset_switch_seconds:
                requested = 8192 if int((time.monotonic()-started)//arguments.wal_offset_switch_seconds)%2 else 0
                if requested!=displacement:
                    displacement=requested
                    emit(dict(kind='wal_offset_switch',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,offset_bytes=displacement))
            if role=='wal' and arguments.wal_cache_switch_seconds:
                requested = arguments.wal_nocache != bool(int((time.monotonic()-started)//arguments.wal_cache_switch_seconds)%2)
                if requested!=nocache:
                    fcntl.fcntl(descriptor,48,int(requested))
                    nocache=requested
                    emit(dict(kind='wal_cache_switch',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,nocache=nocache))
            if role=='wal' and arguments.wal_working_switch_seconds:
                requested = 1024**3 if int((time.monotonic()-started)//arguments.wal_working_switch_seconds)%2 else capacity
                if requested!=working_capacity:
                    working_capacity=requested
                    emit(dict(kind='working_capacity',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed,working_capacity_bytes=working_capacity))
            if arguments.pause_after_seconds and not paused_seconds and time.monotonic()-started>=arguments.pause_after_seconds:
                emit(dict(kind='pause_start',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed))
                began=time.monotonic()
                time.sleep(arguments.pause_seconds)
                paused_seconds=time.monotonic()-began
                emit(dict(kind='pause_end',at=time.time(),elapsed_s=time.monotonic()-started,bytes=completed))
            began = time.monotonic()
            assert os.pwrite(descriptor, block, completed % working_capacity + displacement) == len(block)
            write_end = time.monotonic()
            latencies.append((write_end - began) * 1000)
            if write_end-began > 0.1:
                emit(dict(kind='slow_write', at=time.time(), elapsed_s=write_end-started, duration_s=write_end-began))
            completed += len(block)
            now = time.monotonic()
            if now - previous >= 1:
                sync_ms = 0
                if role == 'data' and now-last_sync>=fsync_seconds:
                    began = time.monotonic()
                    sync_started_at = time.time()
                    os.fsync(descriptor)
                    sync_ms = (time.monotonic() - began) * 1000
                    last_sync = time.monotonic()
                    emit(dict(kind='fsync', at=time.time(), started_at=sync_started_at, duration_ms=sync_ms))
                now = time.monotonic()
                ordered = sorted(latencies)
                emit(dict(kind='sample', at=time.time(), elapsed_s=now-started, bytes=completed, interval_bytes=completed-previous_bytes, interval_s=now-previous, median_ms=ordered[len(ordered)//2], p99_ms=ordered[min(len(ordered)-1,int(len(ordered)*.99))], maximum_ms=max(ordered), fsync_ms=sync_ms))
                previous, previous_bytes, latencies = now, completed, []
                if role=='data' and arguments.data_cache_switch_seconds:
                    requested = bool(int((now-started)//arguments.data_cache_switch_seconds)%2)
                    if requested != nocache:
                        fcntl.fcntl(descriptor,48,int(requested))
                        nocache = requested
                        emit(dict(kind='cache_switch',at=time.time(),elapsed_s=time.monotonic()-started,nocache=nocache))
            # Limit submitted bytes; delayed writes do not raise the lifetime target.
            if rate and completed % (1024 * 1024) == 0:
                if arguments.no_catchup:
                    deadline = max(deadline + 1024**2 / rate, time.monotonic())
                    time.sleep(max(0, deadline - time.monotonic()))
                else:
                    time.sleep(max(0, started + paused_seconds + completed / rate - time.monotonic()))
        began = time.monotonic()
        os.fsync(descriptor)
        emit(dict(kind='final', at=time.time(), elapsed_s=time.monotonic()-started, bytes=completed, final_fsync_ms=(time.monotonic()-began)*1000))
    finally:
        os.close(descriptor)
        path.unlink()
    sys.exit(0)

directory.mkdir(parents=True, exist_ok=False)
(directory/'storage_isolate.py').write_bytes(pathlib.Path(__file__).read_bytes())
(directory/'source.sha256').write_text(hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest())
# macOS SDK sys/resource.h: UUID[16], then eighteen uint64 fields in v2.
library = ctypes.CDLL('/usr/lib/libproc.dylib', use_errno=True)
library.proc_pid_rusage.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
library.proc_pid_rusage.restype = ctypes.c_int
processes = []
files = []
try:
    disk_file = (directory/'iostat.txt').open('w'); files.append(disk_file)
    disk = subprocess.Popen(['/usr/sbin/iostat', '-w', '1'], stdout=disk_file, stderr=subprocess.STDOUT)
    processes.append(disk)
    children = []
    for role in (['wal', 'data'] if arguments.mixed else ['wal']):
        output = (directory/(role+'.jsonl')).open('w'); files.append(output)
        child = subprocess.Popen([sys.executable, __file__, '--worker', role, '--disk-policy-switch-seconds',str(arguments.disk_policy_switch_seconds), '--directory', str(directory), '--seconds', str(arguments.seconds), '--data-gib', str(arguments.data_gib), '--data-block-kib', str(arguments.data_block_kib), '--data-offset-kib', str(arguments.data_offset_kib), '--wal-offset-kib', str(arguments.wal_offset_kib), '--wal-gib', str(arguments.wal_gib),'--wal-rate-mib',str(arguments.wal_rate_mib),'--wal-working-switch-seconds',str(arguments.wal_working_switch_seconds),'--wal-cache-switch-seconds',str(arguments.wal_cache_switch_seconds),'--wal-offset-switch-seconds',str(arguments.wal_offset_switch_seconds),'--data-fsync-seconds',str(arguments.data_fsync_seconds),'--data-fsync-switch-seconds',str(arguments.data_fsync_switch_seconds),'--data-sync-switch-seconds',str(arguments.data_sync_switch_seconds),'--data-cache-switch-seconds',str(arguments.data_cache_switch_seconds),'--data-rate-mib',str(arguments.data_rate_mib),'--data-rate-switch-seconds',str(arguments.data_rate_switch_seconds),'--data-low-rate-mib',str(arguments.data_low_rate_mib),'--pause-after-seconds',str(arguments.pause_after_seconds),'--pause-seconds',str(arguments.pause_seconds)]+(['--wal-nocache'] if arguments.wal_nocache else [])+(['--aligned-buffer'] if arguments.aligned_buffer else [])+(['--no-catchup'] if arguments.no_catchup else [])+(['--wal-prefill'] if arguments.wal_prefill else [])+(['--data-nocache'] if arguments.data_nocache else [])+(['--data-sync'] if arguments.data_sync else [])+(['--extent-map'] if arguments.extent_map else []), stdout=output, stderr=subprocess.STDOUT)
        children.append(child); processes.append(child)
    observer_started = time.monotonic()
    observer_enabled = None
    first_host_sample = True
    with (directory/'host.jsonl').open('w') as output:
        while any(child.poll() is None for child in children):
            free = shutil.disk_usage(directory).free
            assert free > 40 * 1024**3, 'host free-space reserve reached'
            enabled = not arguments.host_sample_switch_seconds or bool(int((time.monotonic()-observer_started)//arguments.host_sample_switch_seconds)%2)
            if enabled != observer_enabled:
                observer_enabled = enabled
                disk.send_signal(signal.SIGCONT if enabled else signal.SIGSTOP)
                with (directory/'observer-switch.jsonl').open('a') as switches:
                    switches.write(json.dumps(dict(at=time.time(), elapsed_s=time.monotonic()-observer_started, enabled=enabled))+'\n')
            if not enabled and not first_host_sample:
                time.sleep(1)
                continue
            first_host_sample = False
            records = []; denied = 0
            for line in subprocess.check_output(['ps', '-axo', 'pid=,comm='], text=True).splitlines():
                fields = line.strip().split(None, 1)
                if len(fields) != 2: continue
                buffer = ctypes.create_string_buffer(160)
                if library.proc_pid_rusage(int(fields[0]), 2, buffer):
                    denied += 1; continue
                values = (ctypes.c_uint64 * 18).from_buffer_copy(buffer.raw[16:])
                records.append(dict(pid=int(fields[0]), command=fields[1], start=values[8], read_bytes=values[16], write_bytes=values[17]))
            device = plistlib.loads(subprocess.check_output(['/usr/sbin/ioreg','-a','-r','-c','IOBlockStorageDriver']))
            volumes = plistlib.loads(subprocess.check_output(['/usr/sbin/ioreg','-a','-r','-c','AppleAPFSVolume']))
            io_stats = [dict(name=item.get('BSD Name',item['IORegistryEntryName']),statistics=item['Statistics']) for item in device+volumes if 'Statistics' in item and (item.get('IOObjectClass')=='IOBlockStorageDriver' or item.get('Role')==['Data'])]
            output.write(json.dumps(dict(at=time.time(), free_bytes=free, unavailable_processes=denied, processes=records, device_io=io_stats, vm_stat=subprocess.check_output(['vm_stat'], text=True)))+'\n'); output.flush()
            time.sleep(1)
    assert all(child.wait() == 0 for child in children), 'storage worker failed'
finally:
    for process in processes:
        if process.poll() is None:
            process.send_signal(signal.SIGCONT)
            process.terminate()
    for process in processes: process.wait()
    for output in files: output.close()
    for role in ['wal', 'data']:
        (directory/(role+'.bin')).unlink(missing_ok=True)
emit(dict(kind='complete', directory=str(directory), mixed=arguments.mixed))
