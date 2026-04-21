const { spawn } = require('child_process');
const fs = require('fs');

const cp = spawn('go', ['run', 'cmd/localmind/main.go'], { cwd: 'd:/Projects/local-mind/localmind/packages/core' });

cp.stderr.on('data', d => fs.appendFileSync('stderr.log', d));

function sendMsg(msg) {
  const buf = Buffer.from(JSON.stringify(msg));
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(buf.length, 0);
  cp.stdin.write(Buffer.concat([lenBuf, buf]));
}

cp.stdout.on('readable', () => {
    let chunk;
    while (null !== (chunk = cp.stdout.read(4))) {
        const len = chunk.readUInt32BE(0);
        const data = cp.stdout.read(len);
        if (data) {
            fs.appendFileSync('stdout.log', data.toString() + '\n');
        }
    }
});

setTimeout(() => {
    sendMsg({ id: '1', timestamp: Date.now(), type: 'ping' });
}, 1000);

setTimeout(() => cp.kill(), 3000);
