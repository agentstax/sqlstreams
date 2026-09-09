import argparse, json, os, pathlib, signal, time

stopping = False
def stop(signum, frame):
    global stopping
    stopping = True
signal.signal(signal.SIGTERM, stop)

parser = argparse.ArgumentParser()
parser.add_argument('--directory', required=True)
parser.add_argument('--seconds', type=int, default=60)
parser.add_argument('--rate', type=float, default=10)
arguments = parser.parse_args()
path = pathlib.Path(arguments.directory) / ('durable-probe-' + str(os.getpid()) + '.bin')
block = os.urandom(1024 * 1024)
capacity = 512 * len(block)
descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_RDWR, 0o600)
try:
    for offset in range(0, capacity, len(block)):
        assert os.pwrite(descriptor, block, offset) == len(block)
    os.fsync(descriptor)
    os.close(descriptor)
    descriptor = os.open(path, os.O_RDWR | os.O_DSYNC)
    started = time.monotonic()
    previous = started
    due = started
    completed = 0
    latencies = []
    print(json.dumps(dict(kind='config', at=time.time(), block_bytes=len(block), file_bytes=capacity, rate=arguments.rate, sync='O_DSYNC')), flush=True)
    while not stopping and time.monotonic() - started < arguments.seconds:
        began = time.monotonic()
        assert os.pwrite(descriptor, block, (completed * len(block)) % capacity) == len(block)
        latencies.append((time.monotonic() - began) * 1000)
        completed += 1
        now = time.monotonic()
        if now - previous >= 1:
            ordered = sorted(latencies)
            print(json.dumps(dict(kind='sample', at=time.time(), elapsed_s=now-started, writes=len(ordered), interval_s=now-previous, median_ms=ordered[len(ordered)//2], p99_ms=ordered[min(len(ordered)-1, int(len(ordered)*.99))], maximum_ms=max(ordered))), flush=True)
            latencies = []
            previous = now
        if arguments.rate:
            due += 1 / arguments.rate
            time.sleep(max(0, due - time.monotonic()))
    print(json.dumps(dict(kind='final', at=time.time(), elapsed_s=time.monotonic()-started, writes=completed, bytes=completed*len(block))), flush=True)
finally:
    os.close(descriptor)
    path.unlink()
