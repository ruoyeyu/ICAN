import { spawn } from 'node:child_process';

process.umask(0o077);
const children = [
    spawn('/app/server', [], { cwd: '/app', env: { ...process.env, PORT: '8080' }, stdio: 'inherit' }),
    spawn(process.execPath, ['server.js'], { cwd: '/app/web', env: { ...process.env, PORT: '3000' }, stdio: 'inherit' }),
];
let stopping = false;
let exitCode = 1;
function stop(code) {
    if (stopping) return;
    stopping = true;
    exitCode = code;
    for (const child of children) child.kill('SIGTERM');
}
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => stop(0));
await Promise.all(children.map(child => new Promise(resolve => {
    child.on('error', () => stop(1));
    child.on('exit', () => stop(1));
    child.once('close', resolve);
})));
process.exitCode = exitCode;
